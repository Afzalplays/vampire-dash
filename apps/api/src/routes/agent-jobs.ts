import type { FastifyPluginAsync } from "fastify";
import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { decryptNodeCredential } from "../lib/node-credentials.js";

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
      if (!job || job.status !== "LEASED" || !job.leaseTokenHash || !job.leaseExpiresAt) return "NOT_FOUND";
      const storedHash = Buffer.from(job.leaseTokenHash, "hex");
      if (storedHash.length !== suppliedHash.length || !timingSafeEqual(storedHash, suppliedHash)) return "INVALID_LEASE";
      if (job.leaseExpiresAt <= new Date()) return "EXPIRED";
      const updated = await tx.nodeAgentJob.updateMany({
        where: { id: job.id, nodeId: node.id, status: "LEASED", leaseTokenHash: job.leaseTokenHash, leaseExpiresAt: { gt: new Date() } },
        data: { status: "ACKNOWLEDGED", leaseTokenHash: null, leaseExpiresAt: null, lastErrorCode: null, lastError: null },
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
