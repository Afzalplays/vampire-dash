import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { authenticate } from "../lib/authenticate.js";

const serverSelect = {
  id: true,
  name: true,
  software: true,
  version: true,
  memoryMb: true,
  diskMb: true,
  cpuPercent: true,
  status: true,
  expiresAt: true,
  createdAt: true,
  node: { select: { name: true, location: true, healthStatus: true, lastSeenAt: true } },
} as const;

export const serverRoutes: FastifyPluginAsync = async (app) => {
  app.get("/", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const servers = await prisma.minecraftServer.findMany({
      where: { OR: [{ ownerId: user.id }, { subusers: { some: { userId: user.id } } }] },
      select: {
        ...serverSelect,
        ownerId: true,
        subusers: {
          where: { userId: user.id },
          select: { id: true, viewConsole: true, power: true, manageFiles: true, manageBackups: true, manageSettings: true },
          take: 1,
        },
      },
      orderBy: { createdAt: "desc" },
    });
    return {
      servers: servers.map(({ ownerId, subusers, ...server }) => ({
        ...server,
        access: ownerId === user.id ? { role: "OWNER", permissions: { viewConsole: true, power: true, manageFiles: true, manageBackups: true, manageSettings: true } } :
          { role: "SUBUSER", permissions: subusers[0] ? {
            viewConsole: subusers[0].viewConsole,
            power: subusers[0].power,
            manageFiles: subusers[0].manageFiles,
            manageBackups: subusers[0].manageBackups,
            manageSettings: subusers[0].manageSettings,
          } : null },
      })),
    };
  });

  app.get("/:serverId", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const params = z.object({ serverId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid server ID" });
    const server = await prisma.minecraftServer.findFirst({
      where: { id: params.data.serverId, OR: [{ ownerId: user.id }, { subusers: { some: { userId: user.id } } }] },
      select: {
        ...serverSelect,
        ownerId: true,
        subusers: {
          where: { userId: user.id },
          select: { viewConsole: true, power: true, manageFiles: true, manageBackups: true, manageSettings: true },
          take: 1,
        },
        _count: { select: { subusers: true, operations: true } },
      },
    });
    if (!server) return reply.code(404).send({ error: "Server not found" });
    const { ownerId, subusers, ...details } = server;
    const permissions = ownerId === user.id ? {
      viewConsole: true, power: true, manageFiles: true, manageBackups: true, manageSettings: true,
    } : subusers[0] ?? {
      viewConsole: false, power: false, manageFiles: false, manageBackups: false, manageSettings: false,
    };
    return { server: { ...details, access: { role: ownerId === user.id ? "OWNER" : "SUBUSER", permissions } } };
  });
};
