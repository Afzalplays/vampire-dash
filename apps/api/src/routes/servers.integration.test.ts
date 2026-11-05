import assert from "node:assert/strict";
import { createHmac, randomBytes } from "node:crypto";
import { after, before, describe, it } from "node:test";
import Fastify from "fastify";
import cookie from "@fastify/cookie";
import rateLimit from "@fastify/rate-limit";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { serverRoutes } from "./servers.js";

const token = () => randomBytes(32).toString("base64url");
const digest = (value: string) => createHmac("sha256", env.SESSION_SECRET).update(value, "utf8").digest("hex");
let app: ReturnType<typeof Fastify>;

before(async () => {
  app = Fastify({ logger: false });
  await app.register(cookie, { secret: env.SESSION_SECRET });
  await app.register(rateLimit);
  await app.register(serverRoutes, { prefix: "/api/servers" });
  await app.ready();
});

after(async () => {
  await app.close();
  await prisma.$disconnect();
});


describe("server console authorization", () => {
  it("denies a subuser without console permission before contacting the node", async () => {
    const suffix = randomBytes(8).toString("hex");
    const owner = await prisma.user.create({ data: { email: `console-owner-${suffix}@example.test`, username: `console_owner_${suffix}`, passwordHash: "test-only-password-hash", emailVerifiedAt: new Date() } });
    const viewer = await prisma.user.create({ data: { email: `console-viewer-${suffix}@example.test`, username: `console_viewer_${suffix}`, passwordHash: "test-only-password-hash", emailVerifiedAt: new Date() } });
    const sessionToken = token();
    const session = await prisma.session.create({ data: { userId: viewer.id, tokenHash: digest(sessionToken), expiresAt: new Date(Date.now() + 60 * 60 * 1000) } });
    const node = await prisma.minecraftNode.create({ data: {
      name: `console-node-${suffix}`, location: "ci", endpoint: `https://console-${suffix}.example.test`,
      tokenHash: digest(token()), enabled: true, healthStatus: "ONLINE", lastSeenAt: new Date(),
      memoryMb: 8192, diskMb: 81920, allocationStart: 25565, allocationEnd: 25575,
    } });
    const server = await prisma.minecraftServer.create({ data: {
      ownerId: owner.id, nodeId: node.id, name: "Console authorization test", software: "PAPER", version: "1.21.1",
      memoryMb: 1024, diskMb: 2048, cpuPercent: 100,
    } });
    const subuser = await prisma.minecraftServerSubuser.create({ data: { serverId: server.id, userId: viewer.id, viewConsole: false } });
    try {
      const response = await app.inject({ method: "GET", url: `/api/servers/${server.id}/console?tail=20`, headers: { cookie: `vd_session=${sessionToken}` } });
      assert.equal(response.statusCode, 403, response.body);
      assert.match(response.json().error, /console.*permission/i);
      await prisma.minecraftServerSubuser.update({ where: { id: subuser.id }, data: { viewConsole: true, consoleCommands: false } });
      const commandResponse = await app.inject({ method: "POST", url: `/api/servers/${server.id}/console`, headers: { cookie: `vd_session=${sessionToken}`, "content-type": "application/json" }, payload: { command: "op attacker" } });
      assert.equal(commandResponse.statusCode, 403, commandResponse.body);
      assert.match(commandResponse.json().error, /console.*permission/i);
    } finally {
      await prisma.minecraftServerSubuser.delete({ where: { id: subuser.id } });
      await prisma.minecraftServer.delete({ where: { id: server.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.session.delete({ where: { id: session.id } });
      await prisma.user.deleteMany({ where: { id: { in: [owner.id, viewer.id] } } });
    }
  });
});

describe("server creation resource and port allocation", () => {
  it("allocates unique ports and enforces CPU, RAM, disk and port capacity", async () => {
    const suffix = randomBytes(8).toString("hex");
    const user = await prisma.user.create({
      data: { email: `server-create-${suffix}@example.test`, username: `server_create_${suffix}`, passwordHash: "test-only-password-hash", emailVerifiedAt: new Date() },
    });
    const sessionToken = token();
    const session = await prisma.session.create({
      data: { userId: user.id, tokenHash: digest(sessionToken), expiresAt: new Date(Date.now() + 60 * 60 * 1000) },
    });
    const plan = await prisma.plan.create({
      data: { slug: `server-create-${suffix}`, name: "CI server plan", pricePaise: 1000n, billingIntervalDays: 30, memoryMb: 1024, diskMb: 2048, cpuPercent: 100, serverSlots: 5, enabled: true },
    });
    const entitlement = await prisma.userPlanEntitlement.create({
      data: { userId: user.id, planId: plan.id, status: "ACTIVE", startsAt: new Date(Date.now() - 60_000), expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000) },
    });
    const node = await prisma.minecraftNode.create({
      data: {
        name: `server-create-node-${suffix}`, location: "ci", endpoint: `https://server-create-${suffix}.example.test`, minecraftAddress: `mc-${suffix}.example.test`,
        tokenHash: digest(token()), enabled: true, healthStatus: "ONLINE", lastSeenAt: new Date(),
        memoryMb: 8192, diskMb: 81920, cpuCores: 8, memoryOverheadMb: 1024, diskOverheadMb: 1024,
        allocationStart: 25565, allocationEnd: 25566,
      },
    });
    const release = await prisma.minecraftSoftwareRelease.create({
      data: { software: "PAPER", minecraftVersion: "1.21.1", softwareVersion: "1.21.1", javaMajor: 21, dockerImage: "itzg/minecraft-server@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", enabled: true },
    });
    const headers = { cookie: `vd_session=${sessionToken}`, "content-type": "application/json" };
    try {
      const first = await app.inject({ method: "POST", url: "/api/servers", headers, payload: { entitlementId: entitlement.id, name: "CI Server One", software: "PAPER", version: "1.21.1" } });
      assert.equal(first.statusCode, 202, first.body);
      assert.equal(first.json().server.allocation.port, 25565);
      assert.equal(first.json().server.node.minecraftAddress, `mc-${suffix}.example.test`);
      const firstOperation = await prisma.minecraftServerOperation.findFirstOrThrow({ where: { serverId: first.json().server.id, type: "PROVISION_SERVER" }, include: { agentJob: true } });
      assert.equal(firstOperation.status, "QUEUED");
      assert.equal(firstOperation.agentJob?.nodeId, node.id);
      assert.equal((firstOperation.payload as { port: number }).port, 25565);
      const second = await app.inject({ method: "POST", url: "/api/servers", headers, payload: { entitlementId: entitlement.id, name: "CI Server Two", software: "PAPER", version: "1.21.1" } });
      assert.equal(second.statusCode, 202, second.body);
      assert.equal(second.json().server.allocation.port, 25566);
      const third = await app.inject({ method: "POST", url: "/api/servers", headers, payload: { entitlementId: entitlement.id, name: "CI Server Three", software: "PAPER", version: "1.21.1" } });
      assert.equal(third.statusCode, 409, third.body);
      assert.match(third.json().error, /public address or TCP port capacity/i);
      const allocations = await prisma.nodePortAllocation.findMany({ where: { nodeId: node.id }, orderBy: { port: "asc" } });
      assert.deepEqual(allocations.map((allocation) => allocation.port), [25565, 25566]);
    } finally {
      await prisma.nodePortAllocation.deleteMany({ where: { nodeId: node.id } });
      await prisma.minecraftServer.deleteMany({ where: { nodeId: node.id } });
      await prisma.minecraftSoftwareRelease.delete({ where: { id: release.id } });
      await prisma.userPlanEntitlement.delete({ where: { id: entitlement.id } });
      await prisma.plan.delete({ where: { id: plan.id } });
      await prisma.minecraftNode.delete({ where: { id: node.id } });
      await prisma.session.delete({ where: { id: session.id } });
      await prisma.user.delete({ where: { id: user.id } });
    }
  });
});
