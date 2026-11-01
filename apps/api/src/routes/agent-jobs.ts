import type { FastifyPluginAsync } from "fastify";
import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { z } from "zod";
import { Prisma } from "@prisma/client";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { decryptNodeCredential } from "../lib/node-credentials.js";


const powerPayloadSchema = z.object({}).strict();
const propertiesPayloadSchema = z.object({
  motd: z.string().max(128).optional(),
  difficulty: z.enum(["peaceful", "easy", "normal", "hard"]).optional(),
  gamemode: z.enum(["survival", "creative", "adventure", "spectator"]).optional(),
  maxPlayers: z.number().int().min(1).max(500).optional(),
  pvp: z.boolean().optional(), onlineMode: z.boolean().optional(), allowFlight: z.boolean().optional(),
  viewDistance: z.number().int().min(2).max(32).optional(),
  simulationDistance: z.number().int().min(2).max(32).optional(),
  spawnProtection: z.number().int().min(0).max(128).optional(),
}).strict().refine((value) => Object.keys(value).length > 0, "At least one property is required");
function validateClaimedPayload(action: string, payload: unknown): boolean {
  if (["START", "STOP", "RESTART"].includes(action)) return powerPayloadSchema.safeParse(payload).success;
  if (action === "UPDATE_PROPERTIES") return propertiesPayloadSchema.safeParse(payload).success;
  return false;
}

const stableJson = (value: unknown): string => {
  if (value === null || typeof value !== "object") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableJson).join(",")}]`;
  const record = value as Record<string, unknown>;
  return `{${Object.keys(record).sort().map((key) => `${JSON.stringify(key)}:${stableJson(record[key])}`).join(",")}}`;
};

const tokenDigest = (token: string) =>
  createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");

export const agentJobRoutes: FastifyPluginAsync = async (app) => {

  app.post("/jobs/:jobId/ack", { config: { rateLimit: { max: 120, timeWindow: "1 minute" } } }, async (request, reply) => {
    const authorization = request.headers.authorization ?? "";
    const match = /^Bearer ([A-Za-z0-9_-]{40,64})$/.exec(authorization);
    if (!match) return reply.code(401).send({ error: "Node bearer authentication required" });
    const node = await prisma.minecraftNode.findUnique({
      where: { tokenHash: tokenDigest(match[1]) },
      select: { id: true, enabled: true, healthStatus: true },
    });
    if (!node) return reply.code(401).send({ error: "Invalid node credentials" });
    if (!node.enabled || node.healthStatus !== "ONLINE") return reply.code(403).send({ error: "Node is not enabled and online" });

    const params = z.object({ jobId: z.string().min(16).max(128).regex(/^[A-Za-z0-9_-]+$/) }).safeParse(request.params);
    const body = z.object({ leaseToken: z.string().regex(/^[A-Za-z0-9_-]{43}$/) }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid job acknowledgement" });
    const suppliedHash = createHash("sha256").update(body.data.leaseToken).digest();
    const outcome = await prisma.$transaction(async (tx) => {
      const job = await tx.nodeAgentJob.findFirst({
        where: { id: params.data.jobId, nodeId: node.id },
        select: { id: true, status: true, leaseTokenHash: true, leaseExpiresAt: true, operationId: true },
      });
      if (!job || !["LEASED", "ACKNOWLEDGED"].includes(job.status) || !job.leaseTokenHash || !job.leaseExpiresAt) return "NOT_FOUND";
      const storedHash = Buffer.from(job.leaseTokenHash, "hex");
      if (storedHash.length !== suppliedHash.length || !timingSafeEqual(storedHash, suppliedHash)) return "INVALID_LEASE";
      const now = new Date();
      if (job.leaseExpiresAt <= now) return "EXPIRED";

      // ACK is idempotent for the same live lease. A node may retry after a
      // network timeout without causing a second state transition.
      if (job.status === "ACKNOWLEDGED") return "ACKNOWLEDGED";

      const updated = await tx.nodeAgentJob.updateMany({
        where: { id: job.id, nodeId: node.id, status: "LEASED", leaseTokenHash: job.leaseTokenHash, leaseExpiresAt: { gt: now } },
        data: { status: "ACKNOWLEDGED", lastErrorCode: null, lastError: null },
      });
      if (updated.count !== 1) return "CONFLICT";
      await tx.minecraftServerOperation.updateMany({
        where: { id: job.operationId, status: "QUEUED" },
        data: { status: "RUNNING", startedAt: new Date() },
      });
      return "ACKNOWLEDGED";
    }, { maxWait: 10000, timeout: 15000 });
    if (outcome === "NOT_FOUND") return reply.code(404).send({ error: "Active job lease not found for this node" });
    if (outcome === "INVALID_LEASE") return reply.code(401).send({ error: "Lease token is invalid" });
    if (outcome === "EXPIRED") return reply.code(409).send({ error: "Job lease has expired" });
    if (outcome === "CONFLICT") return reply.code(409).send({ error: "Job lease changed; claim the job again" });
    return { ok: true, status: "ACKNOWLEDGED" };
  });


  app.post("/jobs/:jobId/renew", { config: { rateLimit: { max: 120, timeWindow: "1 minute" } } }, async (request, reply) => {
    const authorization = request.headers.authorization ?? "";
    const match = /^Bearer ([A-Za-z0-9_-]{40,64})$/.exec(authorization);
    if (!match) return reply.code(401).send({ error: "Node bearer authentication required" });
    const node = await prisma.minecraftNode.findUnique({ where: { tokenHash: tokenDigest(match[1]) }, select: { id: true, enabled: true, healthStatus: true } });
    if (!node) return reply.code(401).send({ error: "Invalid node credentials" });
    if (!node.enabled || node.healthStatus !== "ONLINE") return reply.code(403).send({ error: "Node is not enabled and online" });
    const params = z.object({ jobId: z.string().min(16).max(128).regex(/^[A-Za-z0-9_-]+$/) }).safeParse(request.params);
    const body = z.object({ leaseToken: z.string().regex(/^[A-Za-z0-9_-]{43}$/) }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid lease renewal" });
    const hash = createHash("sha256").update(body.data.leaseToken).digest();
    const job = await prisma.nodeAgentJob.findFirst({ where: { id: params.data.jobId, nodeId: node.id }, select: { id: true, status: true, leaseTokenHash: true, leaseExpiresAt: true } });
    if (!job || !["LEASED", "ACKNOWLEDGED"].includes(job.status) || !job.leaseTokenHash || !job.leaseExpiresAt) return reply.code(404).send({ error: "Active job lease not found" });
    const stored = Buffer.from(job.leaseTokenHash, "hex");
    if (stored.length !== hash.length || !timingSafeEqual(stored, hash)) return reply.code(401).send({ error: "Lease token is invalid" });
    const now = new Date();
    if (job.leaseExpiresAt <= now) return reply.code(409).send({ error: "Lease has expired" });
    const expires = new Date(now.getTime() + 90_000);
    const updated = await prisma.nodeAgentJob.updateMany({ where: { id: job.id, nodeId: node.id, status: job.status, leaseTokenHash: job.leaseTokenHash, leaseExpiresAt: { gt: now } }, data: { leaseExpiresAt: expires } });
    if (updated.count !== 1) return reply.code(409).send({ error: "Lease changed; renew again after reclaiming" });
    return { ok: true, leaseExpiresAt: expires.toISOString() };
  });

  app.post("/jobs/:jobId/result", { config: { rateLimit: { max: 120, timeWindow: "1 minute" } } }, async (request, reply) => {
    const authorization = request.headers.authorization ?? "";
    const match = /^Bearer ([A-Za-z0-9_-]{40,64})$/.exec(authorization);
    if (!match) return reply.code(401).send({ error: "Node bearer authentication required" });
    const node = await prisma.minecraftNode.findUnique({ where: { tokenHash: tokenDigest(match[1]) }, select: { id: true, enabled: true, healthStatus: true } });
    if (!node) return reply.code(401).send({ error: "Invalid node credentials" });
    if (!node.enabled || node.healthStatus !== "ONLINE") return reply.code(403).send({ error: "Node is not enabled and online" });
    const params = z.object({ jobId: z.string().min(16).max(128).regex(/^[A-Za-z0-9_-]+$/) }).safeParse(request.params);
    const body = z.object({
      leaseToken: z.string().regex(/^[A-Za-z0-9_-]{43}$/),
      status: z.enum(["SUCCEEDED", "FAILED"]),
      result: z.record(z.string(), z.unknown()).optional(),
      errorCode: z.string().regex(/^[A-Z0-9_]{1,64}$/).optional(),
      errorMessage: z.string().max(500).optional(),
    }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid job result" });
    if (body.data.result && Buffer.byteLength(JSON.stringify(body.data.result), "utf8") > 8192) return reply.code(400).send({ error: "Result payload is too large" });
    if (body.data.status === "FAILED" && !body.data.errorCode) return reply.code(400).send({ error: "Failed jobs require an error code" });
    const supplied = createHash("sha256").update(body.data.leaseToken).digest();
    const requestHash = createHash("sha256").update(stableJson(body.data), "utf8").digest("hex");
    const outcome = await prisma.$transaction(async (tx) => {
      const job = await tx.nodeAgentJob.findFirst({
        where: { id: params.data.jobId, nodeId: node.id },
        select: {
          id: true, status: true, leaseTokenHash: true, leaseExpiresAt: true,
          operationId: true, resultRequestHash: true,
        },
      });
      if (!job) return "NOT_FOUND";
      if (job.status === "SUCCEEDED" || job.status === "FAILED") {
        return job.resultRequestHash === requestHash ? job.status : "RESULT_CONFLICT";
      }
      if (job.status !== "ACKNOWLEDGED" || !job.leaseTokenHash || !job.leaseExpiresAt) return "NOT_FOUND";
      const stored = Buffer.from(job.leaseTokenHash, "hex");
      if (stored.length !== supplied.length || !timingSafeEqual(stored, supplied)) return "INVALID_LEASE";
      const now = new Date();
      if (job.leaseExpiresAt <= now) return "EXPIRED";
      const updated = await tx.nodeAgentJob.updateMany({
        where: { id: job.id, nodeId: node.id, status: "ACKNOWLEDGED", leaseTokenHash: job.leaseTokenHash, leaseExpiresAt: { gt: now } },
        data: { status: body.data.status, leaseTokenHash: null, leaseExpiresAt: null, resultRequestHash: requestHash, lastErrorCode: body.data.status === "FAILED" ? body.data.errorCode : null, lastError: body.data.status === "FAILED" ? body.data.errorMessage ?? "Node agent reported operation failure" : null },
      });
      if (updated.count !== 1) return "CONFLICT";
      await tx.minecraftServerOperation.updateMany({
        where: { id: job.operationId, status: "RUNNING" },
        data: body.data.status === "SUCCEEDED"
          ? { status: "SUCCEEDED", result: (body.data.result ?? {}) as Prisma.InputJsonValue, finishedAt: now, errorCode: null, errorMessage: null }
          : { status: "FAILED", result: (body.data.result ?? {}) as Prisma.InputJsonValue, errorCode: body.data.errorCode, errorMessage: body.data.errorMessage ?? "Node agent reported operation failure", finishedAt: now },
      });
      return body.data.status;
    }, { maxWait: 10000, timeout: 15000 });
    if (outcome === "NOT_FOUND") return reply.code(404).send({ error: "Acknowledged job not found" });
    if (outcome === "INVALID_LEASE") return reply.code(401).send({ error: "Lease token is invalid" });
    if (outcome === "EXPIRED") return reply.code(409).send({ error: "Lease has expired" });
    if (outcome === "CONFLICT" || outcome === "RESULT_CONFLICT") return reply.code(409).send({ error: "Job state changed or result does not match the original submission" });
    return { ok: true, status: outcome };
  });

  app.post("/jobs/claim", { config: { rateLimit: { max: 60, timeWindow: "1 minute" } } }, async (request, reply) => {
    const authorization = request.headers.authorization ?? "";
    const match = /^Bearer ([A-Za-z0-9_-]{40,64})$/.exec(authorization);
    if (!match) return reply.code(401).send({ error: "Node bearer authentication required" });

    const digest = tokenDigest(match[1]);
    const node = await prisma.minecraftNode.findUnique({
      where: { tokenHash: digest },
      select: {
        id: true, enabled: true, healthStatus: true,
        tokenCiphertext: true, tokenNonce: true, tokenAuthTag: true,
      },
    });
    if (!node) return reply.code(401).send({ error: "Invalid node credentials" });
    if (!node.enabled || node.healthStatus !== "ONLINE") {
      return reply.code(403).send({ error: "Node is not enabled and online" });
    }
    if (!node.tokenCiphertext || !node.tokenNonce || !node.tokenAuthTag) {
      return reply.code(503).send({ error: "Node signing credential is unavailable" });
    }

    const now = new Date();
    const leaseExpiresAt = new Date(now.getTime() + 90_000);
    const leaseToken = randomBytes(32).toString("base64url");
    const leaseTokenHash = createHash("sha256").update(leaseToken).digest("hex");

    const claimed = await prisma.$transaction(async (tx) => {
      const rows = await tx.$queryRaw<Array<{
        id: string;
        nonce: string;
        operationId: string;
        operationType: string;
        payload: unknown;
        attempts: number;
        maxAttempts: number;
      }>>`
        SELECT j."id", j."nonce", j."operationId", o."type" AS "operationType",
               o."payload", j."attempts", j."maxAttempts"
        FROM "NodeAgentJob" j
        INNER JOIN "MinecraftServerOperation" o ON o."id" = j."operationId"
        WHERE j."nodeId" = ${node.id}
          AND j."status" = 'PENDING'
          AND j."availableAt" <= ${now}
          AND j."attempts" < j."maxAttempts"
          AND o."status" = 'QUEUED'
          AND o."type" IN ('START', 'STOP', 'RESTART', 'UPDATE_PROPERTIES')
        ORDER BY j."availableAt" ASC, j."createdAt" ASC
        LIMIT 1
        FOR UPDATE OF j SKIP LOCKED
      `;
      const row = rows[0];
      if (!row) return null;
      const updated = await tx.nodeAgentJob.updateMany({
        where: { id: row.id, status: "PENDING" },
        data: {
          status: "LEASED",
          attempts: { increment: 1 },
          leaseTokenHash,
          leaseExpiresAt,
        },
      });
      if (updated.count !== 1) return null;
      return row;
    }, { maxWait: 10000, timeout: 15000 });

    if (!claimed) return reply.code(204).send();

    if (!validateClaimedPayload(claimed.operationType, claimed.payload)) {
      await prisma.$transaction(async (tx) => {
        await tx.nodeAgentJob.updateMany({ where: { id: claimed.id, nodeId: node.id, status: "LEASED", leaseTokenHash }, data: { status: "DEAD", leaseTokenHash: null, leaseExpiresAt: null, lastErrorCode: "INVALID_JOB_PAYLOAD", lastError: "Queued operation payload failed node-agent schema validation" } });
        await tx.minecraftServerOperation.updateMany({ where: { id: claimed.operationId, status: "QUEUED" }, data: { status: "FAILED", errorCode: "INVALID_JOB_PAYLOAD", errorMessage: "Queued operation payload failed node-agent schema validation", finishedAt: new Date() } });
      });
      return reply.code(409).send({ error: "Queued operation payload is invalid and was rejected" });
    }

    const signingSecret = decryptNodeCredential({
      ciphertext: node.tokenCiphertext,
      nonce: node.tokenNonce,
      authTag: node.tokenAuthTag,
    });
    const issuedAt = new Date();
    const expiresAt = new Date(Math.min(leaseExpiresAt.getTime(), issuedAt.getTime() + 120_000));
    const envelope = {
      id: claimed.id,
      nodeId: node.id,
      nonce: claimed.nonce,
      issuedAt: issuedAt.toISOString(),
      expiresAt: expiresAt.toISOString(),
      action: claimed.operationType,
      payload: claimed.payload,
    };
    const signedBody = JSON.stringify(envelope);
    const signature = createHmac("sha256", signingSecret).update(signedBody, "utf8").digest("hex");

    return reply.code(200).send({
      job: envelope,
      signedBody,
      signature: `sha256=${signature}`,
      leaseToken,
      leaseExpiresAt: leaseExpiresAt.toISOString(),
    });
  });
};
