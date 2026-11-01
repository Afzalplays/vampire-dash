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
  app.post("/", { config: { rateLimit: { max: 5, timeWindow: "1 hour" } } }, async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const parsed = z.object({
      entitlementId: z.string().min(1).max(128),
      name: z.string().trim().min(3).max(32).regex(/^[A-Za-z0-9][A-Za-z0-9 _.-]*$/),
      software: z.enum(["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"]),
      version: z.string().trim().min(1).max(32),
    }).strict().safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid server creation request", details: parsed.error.flatten() });

    const now = new Date();
    try {
      const result = await prisma.$transaction(async (tx) => {
        const entitlement = await tx.userPlanEntitlement.findFirst({
          where: {
            id: parsed.data.entitlementId, userId: user.id, status: "ACTIVE",
            startsAt: { lte: now }, expiresAt: { gt: now }, plan: { enabled: true },
          },
          include: { plan: true },
        });
        if (!entitlement) throw new Error("ENTITLEMENT_NOT_AVAILABLE");

        const duplicateName = await tx.minecraftServer.findFirst({
          where: { ownerId: user.id, name: { equals: parsed.data.name, mode: "insensitive" }, status: { notIn: ["DELETED", "ERROR"] } },
          select: { id: true },
        });
        if (duplicateName) throw new Error("SERVER_NAME_IN_USE");

        const existingCount = await tx.minecraftServer.count({
          where: { ownerId: user.id, status: { notIn: ["DELETED", "ERROR"] }, createdAt: { gte: entitlement.startsAt } },
        });
        if (existingCount >= entitlement.plan.serverSlots) throw new Error("SERVER_SLOT_LIMIT");

        const release = await tx.minecraftSoftwareRelease.findFirst({
          where: { software: parsed.data.software, minecraftVersion: parsed.data.version, enabled: true },
          select: { id: true },
        });
        if (!release) throw new Error("SOFTWARE_RELEASE_UNAVAILABLE");

        const candidates = await tx.$queryRaw<Array<{ id: string }>>`SELECT "id" FROM "MinecraftNode" WHERE "enabled" = true AND "healthStatus" = 'ONLINE' AND "lastSeenAt" > NOW() - INTERVAL '3 minutes' ORDER BY "reservedMemoryMb" ASC, "reservedDiskMb" ASC FOR UPDATE SKIP LOCKED`;
        for (const candidate of candidates) {
          const node = await tx.minecraftNode.findUnique({ where: { id: candidate.id }, select: {
            id: true, name: true, memoryMb: true, diskMb: true, memoryOverheadMb: true, diskOverheadMb: true,
          } });
          if (!node) continue;
          const [allocated, held] = await Promise.all([
            tx.minecraftServer.aggregate({ where: { nodeId: node.id, status: { notIn: ["DELETED", "ERROR"] } }, _sum: { memoryMb: true, diskMb: true } }),
            tx.nodeCapacityReservation.aggregate({ where: { nodeId: node.id, status: "ACTIVE", expiresAt: { gt: now } }, _sum: { memoryMb: true, diskMb: true } }),
          ]);
          const ramUsed = (allocated._sum.memoryMb ?? 0) + (held._sum.memoryMb ?? 0);
          const diskUsed = (allocated._sum.diskMb ?? 0) + (held._sum.diskMb ?? 0);
          if (ramUsed + entitlement.plan.memoryMb > node.memoryMb - node.memoryOverheadMb ||
              diskUsed + entitlement.plan.diskMb > node.diskMb - node.diskOverheadMb) continue;

          const server = await tx.minecraftServer.create({ data: {
            ownerId: user.id, nodeId: node.id, name: parsed.data.name, software: parsed.data.software,
            version: parsed.data.version, memoryMb: entitlement.plan.memoryMb, diskMb: entitlement.plan.diskMb,
            cpuPercent: entitlement.plan.cpuPercent, status: "PENDING",
            expiresAt: entitlement.expiresAt,
          }, select: serverSelect });
          await tx.auditLog.create({ data: {
            actorUserId: user.id, action: "SERVER_CREATED", targetId: server.id,
            details: `Created server ${server.name} on node ${node.name} using entitlement ${entitlement.id}; provisioning is pending`,
          } });
          return server;
        }
        throw new Error("NO_NODE_CAPACITY");
      }, { isolationLevel: "Serializable", maxWait: 10000, timeout: 20000 });
      return reply.code(202).send({ server: result, provisioning: "PENDING_AGENT_DISPATCH" });
    } catch (error) {
      const code = error instanceof Error ? error.message : "";
      if (code === "ENTITLEMENT_NOT_AVAILABLE") return reply.code(403).send({ error: "No active entitlement is available for this plan" });
      if (code === "SERVER_NAME_IN_USE") return reply.code(409).send({ error: "A server with this name already exists" });
      if (code === "SERVER_SLOT_LIMIT") return reply.code(409).send({ error: "Your plan's server slot limit has been reached" });
      if (code === "SOFTWARE_RELEASE_UNAVAILABLE") return reply.code(409).send({ error: "This software and Minecraft version are not currently available" });
      if (code === "NO_NODE_CAPACITY") return reply.code(409).send({ error: "No online node has enough available capacity" });
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2034") return reply.code(409).send({ error: "Capacity changed concurrently; please retry" });
      throw error;
    }
  });

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
