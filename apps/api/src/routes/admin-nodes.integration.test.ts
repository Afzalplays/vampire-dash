import assert from "node:assert/strict";
import { createHmac, randomBytes } from "node:crypto";
import { after, before, describe, it } from "node:test";
import Fastify from "fastify";
import cookie from "@fastify/cookie";
import rateLimit from "@fastify/rate-limit";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { adminRoutes } from "./admin.js";

const secretToken = () => randomBytes(32).toString("base64url");
const digest = (value: string) => createHmac("sha256", env.SESSION_SECRET).update(value, "utf8").digest("hex");
let app: ReturnType<typeof Fastify>;

before(async () => {
  app = Fastify({ logger: false });
  await app.register(cookie, { secret: env.SESSION_SECRET });
  await app.register(rateLimit);
  await app.register(adminRoutes, { prefix: "/api/admin" });
  await app.ready();
});

after(async () => {
  await app.close();
  await prisma.$disconnect();
});

describe("admin node recovery and credential routes", () => {
  it("lists uncertain jobs and blocks credential rotation until acknowledged work is resolved", async () => {
    const admin = await prisma.user.create({
      data: {
        email: `node-admin-${randomBytes(8).toString("hex")}@example.test`,
        username: `node_admin_${randomBytes(8).toString("hex")}`,
        passwordHash: "test-only-password-hash",
        role: "ADMIN",
        emailVerifiedAt: new Date(),
      },
    });
    const sessionToken = secretToken();
    const session = await prisma.session.create({
      data: { userId: admin.id, tokenHash: digest(sessionToken), expiresAt: new Date(Date.now() + 60 * 60 * 1000) },
    });
    const oldToken = secretToken();
    const node = await prisma.minecraftNode.create({
      data: {
        name: "admin-integration-node",
        location: "ci",
        endpoint: `https://admin-node-${randomBytes(6).toString("hex")}.example.test`,
        tokenHash: digest(oldToken),
        enabled: true,
        healthStatus: "ONLINE",
        memoryMb: 8192,
        diskMb: 81920,
      },
    });
    const owner = await prisma.user.create({
      data: {
        email: `node-owner-${randomBytes(8).toString("hex")}@example.test`,
        username: `node_owner_${randomBytes(8).toString("hex")}`,
        passwordHash: "test-only-password-hash",
      },
    });
    const server = await prisma.minecraftServer.create({
      data: {
        ownerId: owner.id, nodeId: node.id, name: "uncertain-server", software: "PAPER",
        version: "1.21.1", memoryMb: 2048, diskMb: 10240, cpuPercent: 100,
      },
    });
    const operation = await prisma.minecraftServerOperation.create({
      data: {
        serverId: server.id, actorUserId: owner.id, type: "START", status: "FAILED",
        idempotencyKey: `admin-${randomBytes(12).toString("hex")}`, payload: {},
        errorCode: "AGENT_JOB_LEASE_LOST", errorMessage: "Acknowledged operation lost its agent lease",
        finishedAt: new Date(),
      },
    });
    const job = await prisma.nodeAgentJob.create({
      data: {
        operationId: operation.id, nodeId: node.id, nonce: randomBytes(32).toString("hex"),
        status: "DEAD", attempts: 1, lastErrorCode: "ACKNOWLEDGED_LEASE_EXPIRED",
        lastError: "Acknowledged job lost its agent lease",
      },
    });
    const headers = { cookie: `vd_session=${sessionToken}`, "content-type": "application/json" };

    try {
      const uncertain = await app.inject({
        method: "GET",
        url: `/api/admin/nodes/${node.id}/uncertain-jobs`,
        headers,
      });
      assert.equal(uncertain.statusCode, 200, uncertain.body);
      assert.equal(uncertain.json().jobs.length, 1);
      assert.equal(uncertain.json().jobs[0].id, job.id);
      assert.equal(uncertain.json().jobs[0].operation.serverId, server.id);
      assert.match(uncertain.json().warning, /Do not retry/i);

      await prisma.nodeAgentJob.update({
        where: { id: job.id },
        data: { status: "ACKNOWLEDGED", leaseExpiresAt: new Date(Date.now() + 60_000) },
      });
      await prisma.minecraftServerOperation.update({
        where: { id: operation.id },
        data: { status: "RUNNING", errorCode: null, errorMessage: null, finishedAt: null },
      });

      const blocked = await app.inject({
        method: "POST",
        url: `/api/admin/nodes/${node.id}/rotate-credentials`,
        headers,
        payload: {},
      });
      assert.equal(blocked.statusCode, 409, blocked.body);
      assert.equal((await prisma.minecraftNode.findUniqueOrThrow({ where: { id: node.id } })).tokenHash, digest(oldToken));

      await prisma.nodeAgentJob.update({
        where: { id: job.id },
        data: { status: "DEAD", leaseTokenHash: null, leaseExpiresAt: null, lastErrorCode: "ACKNOWLEDGED_LEASE_EXPIRED" },
      });
      await prisma.minecraftServerOperation.update({
        where: { id: operation.id },
        data: { status: "FAILED", errorCode: "AGENT_JOB_LEASE_LOST", errorMessage: "Acknowledged operation lost its agent lease", finishedAt: new Date() },
      });

      const rotated = await app.inject({
        method: "POST",
        url: `/api/admin/nodes/${node.id}/rotate-credentials`,
        headers,
        payload: {},
      });
      assert.equal(rotated.statusCode, 200, rotated.body);
      const response = rotated.json();
      assert.match(response.agentToken, /^[A-Za-z0-9_-]{43}$/);
      assert.equal(response.node.enabled, false);
      assert.equal(response.node.healthStatus, "UNKNOWN");
      const stored = await prisma.minecraftNode.findUniqueOrThrow({ where: { id: node.id } });
      assert.notEqual(stored.tokenHash, digest(oldToken));
      assert.ok(stored.tokenCiphertext && stored.tokenNonce && stored.tokenAuthTag);
      assert.equal(await prisma.minecraftNode.findUnique({ where: { tokenHash: digest(oldToken) } }), null);
      assert.ok(await prisma.auditLog.findFirst({ where: { action: "NODE_CREDENTIAL_ROTATED", targetId: node.id } }));
      assert.equal(await prisma.session.findUnique({ where: { id: session.id } }) !== null, true);
    } finally {
      await prisma.minecraftServer.delete({ where: { id: server.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.session.delete({ where: { id: session.id } });
      await prisma.user.delete({ where: { id: owner.id } });
      await prisma.user.delete({ where: { id: admin.id } });
    }
  });
});
