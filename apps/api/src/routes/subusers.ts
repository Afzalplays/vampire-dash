import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { authenticate } from "../lib/authenticate.js";

const permissions = z.object({
  viewConsole: z.boolean().optional(),
  consoleCommands: z.boolean().optional(),
  power: z.boolean().optional(),
  manageFiles: z.boolean().optional(),
  manageBackups: z.boolean().optional(),
  manageSettings: z.boolean().optional(),
}).strict().refine(value => Object.keys(value).length > 0, "At least one permission is required");

export const subuserRoutes: FastifyPluginAsync = async (app) => {
  app.get("/:serverId/subusers", async (request, reply) => {
    const actor = await authenticate(request, reply);
    if (!actor) return;
    const params = z.object({ serverId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid server ID" });
    const server = await prisma.minecraftServer.findFirst({ where: { id: params.data.serverId, ownerId: actor.id }, select: { id: true } });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const subusers = await prisma.minecraftServerSubuser.findMany({
      where: { serverId: server.id },
      select: { id: true, user: { select: { id: true, username: true, email: true } }, viewConsole: true, power: true, manageFiles: true, manageBackups: true, manageSettings: true, createdAt: true },
      orderBy: { createdAt: "asc" },
    });
    return { subusers };
  });

  app.post("/:serverId/subusers", { config: { rateLimit: { max: 10, timeWindow: "1 hour" } } }, async (request, reply) => {
    const actor = await authenticate(request, reply);
    if (!actor) return;
    const params = z.object({ serverId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = z.object({ email: z.string().email().max(254), permissions: permissions.default({ viewConsole: true }) }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid subuser request" });
    const server = await prisma.minecraftServer.findFirst({ where: { id: params.data.serverId, ownerId: actor.id }, select: { id: true } });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const target = await prisma.user.findUnique({ where: { email: body.data.email.toLowerCase() }, select: { id: true, emailVerifiedAt: true, status: true } });
    if (!target || target.status !== "ACTIVE" || !target.emailVerifiedAt) return reply.code(404).send({ error: "An existing active, email-verified account is required" });
    if (target.id === actor.id) return reply.code(400).send({ error: "Server owner cannot be added as a subuser" });
    try {
      const subuser = await prisma.$transaction(async tx => {
        const created = await tx.minecraftServerSubuser.create({ data: { serverId: server.id, userId: target.id, ...body.data.permissions } });
        await tx.auditLog.create({ data: { actorUserId: actor.id, action: "SERVER_SUBUSER_ADDED", targetId: created.id, details: `Added subuser ${target.id} to server ${server.id}` } });
        return created;
      });
      return reply.code(201).send({ subuser });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "User is already a subuser" });
      throw error;
    }
  });

  app.patch("/:serverId/subusers/:subuserId", async (request, reply) => {
    const actor = await authenticate(request, reply);
    if (!actor) return;
    const params = z.object({ serverId: z.string().min(1).max(128), subuserId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = permissions.safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid subuser update" });
    const server = await prisma.minecraftServer.findFirst({ where: { id: params.data.serverId, ownerId: actor.id }, select: { id: true } });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const existing = await prisma.minecraftServerSubuser.findFirst({ where: { id: params.data.subuserId, serverId: server.id }, select: { id: true } });
    if (!existing) return reply.code(404).send({ error: "Subuser not found" });
    const subuser = await prisma.$transaction(async tx => {
      const updated = await tx.minecraftServerSubuser.update({ where: { id: existing.id }, data: body.data });
      await tx.auditLog.create({ data: { actorUserId: actor.id, action: "SERVER_SUBUSER_PERMISSIONS_UPDATED", targetId: updated.id, details: `Updated subuser permissions for server ${server.id}` } });
      return updated;
    });
    return { subuser };
  });

  app.delete("/:serverId/subusers/:subuserId", async (request, reply) => {
    const actor = await authenticate(request, reply);
    if (!actor) return;
    const params = z.object({ serverId: z.string().min(1).max(128), subuserId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid subuser request" });
    const server = await prisma.minecraftServer.findFirst({ where: { id: params.data.serverId, ownerId: actor.id }, select: { id: true } });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const existing = await prisma.minecraftServerSubuser.findFirst({ where: { id: params.data.subuserId, serverId: server.id }, select: { id: true } });
    if (!existing) return reply.code(404).send({ error: "Subuser not found" });
    await prisma.$transaction([
      prisma.minecraftServerSubuser.delete({ where: { id: existing.id } }),
      prisma.auditLog.create({ data: { actorUserId: actor.id, action: "SERVER_SUBUSER_REMOVED", targetId: existing.id, details: `Removed subuser from server ${server.id}` } }),
    ]);
    return { ok: true };
  });
};
