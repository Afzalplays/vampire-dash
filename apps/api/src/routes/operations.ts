import type { FastifyPluginAsync } from "fastify";
import { randomBytes } from "node:crypto";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { authenticate } from "../lib/authenticate.js";

const properties = z.object({
  motd: z.string().max(128).optional(),
  difficulty: z.enum(["peaceful", "easy", "normal", "hard"]).optional(),
  gamemode: z.enum(["survival", "creative", "adventure", "spectator"]).optional(),
  maxPlayers: z.number().int().min(1).max(500).optional(),
  pvp: z.boolean().optional(),
  onlineMode: z.boolean().optional(),
  allowFlight: z.boolean().optional(),
  viewDistance: z.number().int().min(2).max(32).optional(),
  simulationDistance: z.number().int().min(2).max(32).optional(),
  spawnProtection: z.number().int().min(0).max(128).optional(),
}).strict().refine((value) => Object.keys(value).length > 0, "At least one property is required");

const operationRequest = z.discriminatedUnion("type", [
  z.object({ type: z.enum(["START", "STOP", "RESTART"]), payload: z.object({}).strict(), idempotencyKey: z.string().min(16).max(128).regex(/^[A-Za-z0-9._:-]+$/) }).strict(),
  z.object({ type: z.literal("UPDATE_PROPERTIES"), payload: properties, idempotencyKey: z.string().min(16).max(128).regex(/^[A-Za-z0-9._:-]+$/) }).strict(),
]);

export const operationRoutes: FastifyPluginAsync = async (app) => {
  app.post("/:serverId/operations", { config: { rateLimit: { max: 20, timeWindow: "1 minute" } } }, async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const params = z.object({ serverId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = operationRequest.safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid server operation", details: body.success ? undefined : body.error.flatten() });

    if (body.data.type === "UPDATE_PROPERTIES") return reply.code(409).send({ error: "Server properties updates are not available until the verified file adapter is enabled" });

    const server = await prisma.minecraftServer.findUnique({
      where: { id: params.data.serverId },
      select: { id: true, ownerId: true, nodeId: true, status: true },
    });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const isOwner = server.ownerId === user.id;
    const membership = isOwner ? null : await prisma.minecraftServerSubuser.findUnique({
      where: { serverId_userId: { serverId: server.id, userId: user.id } },
      select: { power: true, manageSettings: true },
    });
    const allowed = isOwner || (membership && (body.data.type === "UPDATE_PROPERTIES" ? membership.manageSettings : membership.power));
    if (!allowed) return reply.code(403).send({ error: "You do not have permission for this operation" });

    const existing = await prisma.minecraftServerOperation.findUnique({ where: { idempotencyKey: body.data.idempotencyKey } });
    if (existing) {
      if (existing.serverId !== server.id || existing.actorUserId !== user.id || existing.type !== body.data.type) {
        return reply.code(409).send({ error: "Idempotency key was already used for a different operation" });
      }
      return reply.send({ operation: existing, reused: true, execution: "NOT_DISPATCHED" });
    }

    if (!server.nodeId) return reply.code(409).send({ error: "Server is not assigned to a node yet" });
    if (["SUSPENDED", "DELETING", "DELETED", "ERROR"].includes(server.status)) {
      return reply.code(409).send({ error: "Server state does not allow operations" });
    }
    const active = await prisma.minecraftServerOperation.findFirst({
      where: { serverId: server.id, status: { in: ["QUEUED", "RUNNING"] } },
      select: { id: true },
    });
    if (active) return reply.code(409).send({ error: "Another server operation is already queued or running", operationId: active.id });

    try {
      const operation = await prisma.$transaction(async (tx) => {
        const created = await tx.minecraftServerOperation.create({
          data: {
            serverId: server.id,
            actorUserId: user.id,
            type: body.data.type,
            idempotencyKey: body.data.idempotencyKey,
            payload: body.data.payload,
          },
        });
        await tx.nodeAgentJob.create({
          data: { operationId: created.id, nodeId: server.nodeId!, nonce: randomBytes(32).toString("hex"), status: "PENDING" },
        });
        await tx.auditLog.create({
          data: { actorUserId: user.id, action: "SERVER_OPERATION_QUEUED", targetId: created.id, details: `Queued ${created.type} for server ${server.id}; agent job pending secure dispatch` },
        });
        return created;
      });
      return reply.code(202).send({ operation, reused: false, execution: "NOT_DISPATCHED" });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") {
        const duplicate = await prisma.minecraftServerOperation.findUnique({ where: { idempotencyKey: body.data.idempotencyKey } });
        if (duplicate && duplicate.serverId === server.id && duplicate.actorUserId === user.id && duplicate.type === body.data.type) {
          return { operation: duplicate, reused: true, execution: "NOT_DISPATCHED" };
        }
        return reply.code(409).send({ error: "Idempotency key conflict" });
      }
      throw error;
    }
  });

  app.get("/:serverId/operations", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const params = z.object({ serverId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid server ID" });
    const server = await prisma.minecraftServer.findUnique({ where: { id: params.data.serverId }, select: { id: true, ownerId: true } });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const isOwner = server.ownerId === user.id;
    const membership = isOwner ? null : await prisma.minecraftServerSubuser.findUnique({ where: { serverId_userId: { serverId: server.id, userId: user.id } }, select: { viewConsole: true } });
    if (!isOwner && !membership?.viewConsole) return reply.code(403).send({ error: "You do not have permission to view operations" });
    const operations = await prisma.minecraftServerOperation.findMany({
      where: { serverId: server.id },
      select: { id: true, type: true, status: true, payload: true, result: true, errorCode: true, errorMessage: true, createdAt: true, startedAt: true, finishedAt: true },
      orderBy: { createdAt: "desc" },
      take: 50,
    });
    return { operations, execution: "NOT_DISPATCHED" };
  });
};
