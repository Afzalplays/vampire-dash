import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";

const softwareTypes = ["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"] as const;
const querySchema = z.object({
  software: z.enum(softwareTypes).optional(),
  minecraftVersion: z.string().trim().min(1).max(32).optional(),
});

export const softwareRoutes: FastifyPluginAsync = async (app) => {
  app.get("/releases", async (request, reply) => {
    const parsed = querySchema.safeParse(request.query);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid software catalogue filters" });
    const releases = await prisma.minecraftSoftwareRelease.findMany({
      where: { enabled: true, ...(parsed.data.software ? { software: parsed.data.software } : {}), ...(parsed.data.minecraftVersion ? { minecraftVersion: parsed.data.minecraftVersion } : {}) },
      select: { id: true, software: true, minecraftVersion: true, softwareVersion: true, javaMajor: true, dockerImage: true, downloadUrl: true, checksumSha256: true, recommended: true },
      orderBy: [{ recommended: "desc" }, { software: "asc" }, { minecraftVersion: "desc" }, { softwareVersion: "desc" }],
      take: 200,
    });
    return { releases };
  });
};
