import type { FastifyPluginAsync } from "fastify";
import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { decryptNodeCredential } from "../lib/node-credentials.js";

const tokenDigest = (token: string) =>
  createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");

export const agentJobRoutes: FastifyPluginAsync = async (app) => {
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
