import assert from "node:assert/strict";
import { createHash, createHmac, randomBytes } from "node:crypto";
import { after, before, describe, it } from "node:test";
import Fastify from "fastify";
import rateLimit from "@fastify/rate-limit";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { nodeAgentTerminalResultHash, nodeOperationIdentityHash } from "../lib/agent-job-identity.js";
import { agentJobRoutes } from "./agent-jobs.js";
import { recoverExpiredAgentJobs } from "../workers/agent-job-recovery.js";

const token = Buffer.from(randomBytes(32)).toString("base64url");
const tokenHash = createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");
let app: ReturnType<typeof Fastify>;

before(async () => {
  app = Fastify({ logger: false });
  await app.register(rateLimit);
  await app.register(agentJobRoutes, { prefix: "/api/agent" });
  await app.ready();
});

after(async () => {
  await app.close();
  await prisma.$disconnect();
});



describe("server lifecycle after expired agent leases", () => {
  it("marks acknowledged provisioning UNKNOWN and exhausted unacknowledged provisioning ERROR", async () => {
    const suffix = randomBytes(8).toString("hex");
    const user = await prisma.user.create({ data: { email: `lifecycle-${suffix}@example.test`, username: `lifecycle_${suffix}`, passwordHash: "test-only-password-hash" } });
    const node = await prisma.minecraftNode.create({ data: {
      name: `lifecycle-node-${suffix}`, location: "ci", endpoint: `https://lifecycle-${suffix}.example.test`,
      tokenHash: createHmac("sha256", env.SESSION_SECRET).update(randomBytes(32)).digest("hex"),
      enabled: true, healthStatus: "ONLINE", memoryMb: 8192, diskMb: 81920,
    } });
    const acknowledgedServer = await prisma.minecraftServer.create({ data: {
      ownerId: user.id, nodeId: node.id, name: "Acknowledged provision", software: "PAPER", version: "1.21.1",
      memoryMb: 1024, diskMb: 2048, cpuPercent: 100, status: "PENDING",
    } });
    const exhaustedServer = await prisma.minecraftServer.create({ data: {
      ownerId: user.id, nodeId: node.id, name: "Exhausted provision", software: "PAPER", version: "1.21.1",
      memoryMb: 1024, diskMb: 2048, cpuPercent: 100, status: "PENDING",
    } });
    const acknowledgedOperation = await prisma.minecraftServerOperation.create({ data: {
      serverId: acknowledgedServer.id, actorUserId: user.id, type: "PROVISION_SERVER", status: "RUNNING",
      idempotencyKey: `lifecycle-ack-${suffix}`, payload: {},
    } });
    const exhaustedOperation = await prisma.minecraftServerOperation.create({ data: {
      serverId: exhaustedServer.id, actorUserId: user.id, type: "PROVISION_SERVER", status: "QUEUED",
      idempotencyKey: `lifecycle-exhausted-${suffix}`, payload: {},
    } });
    const expired = new Date(Date.now() - 60_000);
    const acknowledgedJob = await prisma.nodeAgentJob.create({ data: {
      operationId: acknowledgedOperation.id, nodeId: node.id, nonce: randomBytes(32).toString("hex"),
      status: "ACKNOWLEDGED", attempts: 1, leaseTokenHash: randomBytes(32).toString("hex"), leaseExpiresAt: expired,
    } });
    const exhaustedJob = await prisma.nodeAgentJob.create({ data: {
      operationId: exhaustedOperation.id, nodeId: node.id, nonce: randomBytes(32).toString("hex"),
      status: "LEASED", attempts: 5, maxAttempts: 5, leaseTokenHash: randomBytes(32).toString("hex"), leaseExpiresAt: expired,
    } });
    try {
      const recovered = await recoverExpiredAgentJobs(new Date());
      assert.equal(recovered.dead >= 2, true);
      assert.equal((await prisma.minecraftServer.findUniqueOrThrow({ where: { id: acknowledgedServer.id } })).status, "UNKNOWN");
      assert.equal((await prisma.minecraftServer.findUniqueOrThrow({ where: { id: exhaustedServer.id } })).status, "ERROR");
      assert.equal((await prisma.nodeAgentJob.findUniqueOrThrow({ where: { id: acknowledgedJob.id } })).status, "DEAD");
      assert.equal((await prisma.nodeAgentJob.findUniqueOrThrow({ where: { id: exhaustedJob.id } })).status, "DEAD");
    } finally {
      await prisma.minecraftServer.deleteMany({ where: { id: { in: [acknowledgedServer.id, exhaustedServer.id] } } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.user.delete({ where: { id: user.id } });
    }
  });
});

describe("agent reconciliation route database integration", () => {
  it("accepts a matching pending result once and rejects identity conflicts", async () => {
    const user = await prisma.user.create({
      data: {
        email: `agent-route-${randomBytes(8).toString("hex")}@example.test`,
        username: `agent_route_${randomBytes(8).toString("hex")}`,
        passwordHash: "test-only-password-hash",
      },
    });
    const node = await prisma.minecraftNode.create({
      data: {
        name: "integration-node",
        location: "ci",
        endpoint: "https://node.example.test",
        tokenHash,
        enabled: true,
        healthStatus: "ONLINE",
        memoryMb: 8192,
        diskMb: 81920,
      },
    });
    const server = await prisma.minecraftServer.create({
      data: {
        ownerId: user.id,
        nodeId: node.id,
        name: "integration-server",
        status: "UNKNOWN",
        software: "PAPER",
        version: "1.21.1",
        memoryMb: 2048,
        diskMb: 10240,
        cpuPercent: 100,
      },
    });
    const operation = await prisma.minecraftServerOperation.create({
      data: {
        serverId: server.id,
        actorUserId: user.id,
        type: "PROVISION_SERVER",
        status: "FAILED",
        idempotencyKey: `integration-${randomBytes(12).toString("hex")}`,
        payload: {},
        errorCode: "AGENT_JOB_LEASE_LOST",
        errorMessage: "Acknowledged operation lost its agent lease",
        finishedAt: new Date(),
      },
    });
    const job = await prisma.nodeAgentJob.create({
      data: {
        operationId: operation.id,
        nodeId: node.id,
        nonce: randomBytes(32).toString("hex"),
        status: "DEAD",
        attempts: 1,
        lastErrorCode: "ACKNOWLEDGED_LEASE_EXPIRED",
        lastError: "Acknowledged job lost its agent lease",
      },
    });

    try {
      const terminalResult = { status: "SUCCEEDED", result: { observedState: "RUNNING" } };
      const identityHash = nodeOperationIdentityHash({
        id: job.id,
        nodeId: node.id,
        nonce: job.nonce,
        operation: { serverId: server.id, type: operation.type, payload: operation.payload },
      });
      const resultHash = nodeAgentTerminalResultHash(terminalResult);
      const url = `/api/agent/jobs/${job.id}/reconcile`;
      const headers = { authorization: `Bearer ${token}`, "content-type": "application/json" };
      const body = { identityHash, state: "RESULT_PENDING", resultHash, terminalResult };

      const accepted = await app.inject({ method: "POST", url, headers, payload: body });
      assert.equal(accepted.statusCode, 200, accepted.body);
      assert.equal(accepted.json().status, "RESULT_ACCEPTED");

      const committedJob = await prisma.nodeAgentJob.findUniqueOrThrow({ where: { id: job.id } });
      const committedOperation = await prisma.minecraftServerOperation.findUniqueOrThrow({ where: { id: operation.id } });
      assert.equal(committedJob.status, "SUCCEEDED");
      assert.equal(committedJob.resultRequestHash, resultHash);
      assert.equal(committedOperation.status, "SUCCEEDED");
      assert.deepEqual(committedOperation.result, terminalResult.result);
      assert.equal((await prisma.minecraftServer.findUniqueOrThrow({ where: { id: server.id } })).status, "RUNNING");

      const duplicate = await app.inject({ method: "POST", url, headers, payload: body });
      assert.equal(duplicate.statusCode, 200, duplicate.body);
      assert.equal(duplicate.json().status, "RESULT_COMMITTED");

      const conflict = await app.inject({
        method: "POST",
        url,
        headers,
        payload: { ...body, identityHash: "0".repeat(64) },
      });
      assert.equal(conflict.statusCode, 409, conflict.body);
    } finally {
      await prisma.minecraftServer.delete({ where: { id: server.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.user.delete({ where: { id: user.id } });
    }
  });

  it("marks a provisioned server RUNNING after a valid node result", async () => {
    const user = await prisma.user.create({ data: { email: `agent-provision-${randomBytes(8).toString("hex")}@example.test`, username: `agent_provision_${randomBytes(8).toString("hex")}`, passwordHash: "test-only-password-hash" } });
    const agentToken = Buffer.from(randomBytes(32)).toString("base64url");
    const node = await prisma.minecraftNode.create({ data: { name: "provision-result-node", location: "ci", endpoint: `https://provision-result-${randomBytes(6).toString("hex")}.example.test`, tokenHash: createHmac("sha256", env.SESSION_SECRET).update(agentToken).digest("hex"), enabled: true, healthStatus: "ONLINE", memoryMb: 8192, diskMb: 81920 } });
    const server = await prisma.minecraftServer.create({ data: { ownerId: user.id, nodeId: node.id, name: "provision-result-server", software: "PAPER", version: "1.21.1", memoryMb: 2048, diskMb: 10240, cpuPercent: 100 } });
    const operation = await prisma.minecraftServerOperation.create({ data: { serverId: server.id, actorUserId: user.id, type: "PROVISION_SERVER", status: "RUNNING", idempotencyKey: `provision-result-${randomBytes(12).toString("hex")}`, payload: {}, startedAt: new Date() } });
    const leaseToken = Buffer.from(randomBytes(32)).toString("base64url");
    const job = await prisma.nodeAgentJob.create({ data: { operationId: operation.id, nodeId: node.id, nonce: randomBytes(32).toString("hex"), status: "ACKNOWLEDGED", attempts: 1, leaseTokenHash: createHash("sha256").update(leaseToken).digest("hex"), leaseExpiresAt: new Date(Date.now() + 60_000) } });
    try {
      const response = await app.inject({ method: "POST", url: `/api/agent/jobs/${job.id}/result`, headers: { authorization: `Bearer ${agentToken}`, "content-type": "application/json" }, payload: { leaseToken, status: "SUCCEEDED", result: { state: "RUNNING", port: 25565 } } });
      assert.equal(response.statusCode, 200, response.body);
      assert.equal(response.json().status, "SUCCEEDED");
      assert.equal((await prisma.minecraftServer.findUniqueOrThrow({ where: { id: server.id } })).status, "RUNNING");
      assert.equal((await prisma.minecraftServerOperation.findUniqueOrThrow({ where: { id: operation.id } })).status, "SUCCEEDED");
    } finally {
      await prisma.minecraftServer.delete({ where: { id: server.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.user.delete({ where: { id: user.id } });
    }
  });
  it("reports a live acknowledged lease as in progress without changing it", async () => {
    const user = await prisma.user.create({
      data: {
        email: `agent-live-${randomBytes(8).toString("hex")}@example.test`,
        username: `agent_live_${randomBytes(8).toString("hex")}`,
        passwordHash: "test-only-password-hash",
      },
    });
    const liveToken = token + "live";
    const node = await prisma.minecraftNode.create({
      data: {
        name: "integration-live-node",
        location: "ci",
        endpoint: "https://live-node.example.test",
        tokenHash: createHmac("sha256", env.SESSION_SECRET).update(liveToken).digest("hex"),
        enabled: true,
        healthStatus: "ONLINE",
        memoryMb: 8192,
        diskMb: 81920,
      },
    });
    const server = await prisma.minecraftServer.create({
      data: {
        ownerId: user.id, nodeId: node.id, name: "live-server", software: "PAPER",
        version: "1.21.1", memoryMb: 2048, diskMb: 10240, cpuPercent: 100,
      },
    });
    const operation = await prisma.minecraftServerOperation.create({
      data: {
        serverId: server.id, actorUserId: user.id, type: "START", status: "RUNNING",
        idempotencyKey: `live-${randomBytes(12).toString("hex")}`, payload: {}, startedAt: new Date(),
      },
    });
    const job = await prisma.nodeAgentJob.create({
      data: {
        operationId: operation.id, nodeId: node.id, nonce: randomBytes(32).toString("hex"),
        status: "ACKNOWLEDGED", attempts: 1, leaseTokenHash: "0".repeat(64),
        leaseExpiresAt: new Date(Date.now() + 60_000),
      },
    });
    try {
      const identityHash = nodeOperationIdentityHash({
        id: job.id, nodeId: node.id, nonce: job.nonce,
        operation: { serverId: server.id, type: operation.type, payload: operation.payload },
      });
      const response = await app.inject({
        method: "POST",
        url: `/api/agent/jobs/${job.id}/reconcile`,
        headers: { authorization: `Bearer ${liveToken}`, "content-type": "application/json" },
        payload: { identityHash, state: "EXECUTION_STARTED" },
      });
      assert.equal(response.statusCode, 200, response.body);
      assert.equal(response.json().status, "IN_PROGRESS");
      const unchanged = await prisma.nodeAgentJob.findUniqueOrThrow({ where: { id: job.id } });
      assert.equal(unchanged.status, "ACKNOWLEDGED");
    } finally {
      await prisma.minecraftServer.delete({ where: { id: server.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.user.delete({ where: { id: user.id } });
    }
  });
});
