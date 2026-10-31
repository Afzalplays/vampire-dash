import type { FastifyPluginAsync } from "fastify";
import { createHmac, randomBytes } from "node:crypto";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { requireAdmin } from "../lib/authenticate.js";
import { listPaymentProviders } from "../billing/registry.js";

const nodeCreateSchema = z.object({
  name: z.string().trim().min(2).max(64),
  location: z.string().trim().min(2).max(64),
  endpoint: z.string().url().max(255).refine((value) => {
    try {
      const url = new URL(value);
      return url.protocol === "https:" && !url.username && !url.password && !url.search && !url.hash;
    } catch {
      return false;
    }
  }, "Node endpoint must be an HTTPS URL without credentials, query, or fragment"),
  memoryMb: z.number().int().positive().max(4_194_304),
  diskMb: z.number().int().positive().max(16_777_216),
});
const nodeTokenDigest = (token: string) => createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");

export const adminRoutes: FastifyPluginAsync = async (app) => {
  app.get("/users", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const users = await prisma.user.findMany({ select: { id: true, email: true, username: true, role: true, status: true, createdAt: true }, orderBy: { createdAt: "desc" }, take: 100 });
    return { users };
  });

  app.get("/payment-providers", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    return { defaultProvider: "famgateway", availableProviders: listPaymentProviders() };
  });

  app.get("/nodes", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const nodes = await prisma.minecraftNode.findMany({
      select: { id: true, name: true, location: true, endpoint: true, enabled: true, memoryMb: true, diskMb: true, createdAt: true, _count: { select: { servers: true } } },
      orderBy: { createdAt: "desc" },
    });
    return { nodes };
  });

  app.post("/nodes", { config: { rateLimit: { max: 5, timeWindow: "1 hour" } } }, async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const parsed = nodeCreateSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid node details", details: parsed.error.flatten() });
    const endpoint = new URL(parsed.data.endpoint).toString().replace(/\/$/, "");
    const duplicate = await prisma.minecraftNode.findFirst({ where: { endpoint }, select: { id: true } });
    if (duplicate) return reply.code(409).send({ error: "A node with this endpoint is already registered" });

    const token = randomBytes(32).toString("base64url");
    const node = await prisma.minecraftNode.create({
      data: { ...parsed.data, endpoint, tokenHash: nodeTokenDigest(token), enabled: false },
      select: { id: true, name: true, location: true, endpoint: true, enabled: true, memoryMb: true, diskMb: true, createdAt: true },
    });
    await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_REGISTERED", targetId: node.id, details: `Registered node ${node.name} at ${node.endpoint}; disabled pending secure agent enrollment` } });
    // The raw credential is returned once. It is never persisted or included in later list responses.
    return reply.code(201).send({ node, agentToken: token, warning: "Copy this token now. It will not be shown again. Node remains disabled until mTLS enrollment is implemented." });
  });

  app.post("/nodes/:nodeId/disable", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const params = z.object({ nodeId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid node ID" });
    const node = await prisma.minecraftNode.findUnique({ where: { id: params.data.nodeId }, select: { id: true, name: true, enabled: true } });
    if (!node) return reply.code(404).send({ error: "Node not found" });
    await prisma.minecraftNode.update({ where: { id: node.id }, data: { enabled: false } });
    await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_DISABLED", targetId: node.id, details: `Disabled node ${node.name}` } });
    return { ok: true, enabled: false };
  });

  app.post("/users/:userId/wallet-adjustment", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const params = z.object({ userId: z.string().min(1) }).safeParse(request.params);
    const body = z.object({ amountPaise: z.number().int().min(-100000000).max(100000000).refine(v => v !== 0), reason: z.string().min(5).max(300) }).safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid adjustment" });
    const wallet = await prisma.wallet.findUnique({ where: { userId: params.data.userId } });
    if (!wallet) return reply.code(404).send({ error: "Wallet not found" });
    const result = await prisma.$transaction(async (tx) => {
      const updated = await tx.wallet.updateMany({ where: { id: wallet.id, ...(body.data.amountPaise < 0 ? { balancePaise: { gte: BigInt(-body.data.amountPaise) } } : {}) }, data: { balancePaise: { increment: BigInt(body.data.amountPaise) } } });
      if (updated.count !== 1) throw new Error("Insufficient wallet balance");
      await tx.walletLedger.create({ data: { walletId: wallet.id, amountPaise: BigInt(body.data.amountPaise), kind: "ADMIN_ADJUSTMENT", description: body.data.reason } });
      await tx.auditLog.create({ data: { actorUserId: admin.id, action: "WALLET_ADJUSTMENT", targetId: params.data.userId, details: body.data.reason } });
      return tx.wallet.findUniqueOrThrow({ where: { id: wallet.id }, select: { balancePaise: true, currency: true } });
    });
    return { wallet: { balancePaise: result.balancePaise.toString(), currency: result.currency } };
  });
};
