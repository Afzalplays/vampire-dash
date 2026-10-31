import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";

export const artifactRoutes: FastifyPluginAsync = async (app) => {
  app.get("/", async (request, reply) => {
    const parsed = z.object({
      kind: z.enum(["PLUGIN", "MOD", "MODPACK"]).optional(),
      software: z.enum(["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"]).optional(),
      minecraftVersion: z.string().trim().min(1).max(32).optional(),
      loader: z.string().trim().min(1).max(32).optional(),
    }).safeParse(request.query);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid artifact catalogue filters" });
    const projects = await prisma.minecraftArtifactProject.findMany({
      where: {
        enabled: true,
        ...(parsed.data.kind ? { kind: parsed.data.kind } : {}),
        ...(parsed.data.software ? { supportedSoftware: { has: parsed.data.software } } : {}),
        ...(parsed.data.minecraftVersion ? { supportedGameVersions: { has: parsed.data.minecraftVersion } } : {}),
        ...(parsed.data.loader ? { loaders: { has: parsed.data.loader } } : {}),
      },
      select: {
        id: true, kind: true, provider: true, providerProjectId: true, slug: true, title: true, summary: true,
        projectUrl: true, sourceUrl: true, licenseId: true, supportedSoftware: true, supportedGameVersions: true, loaders: true,
        files: { where: { enabled: true }, select: { id: true, providerFileId: true, versionNumber: true, downloadUrl: true, sha256: true, sizeBytes: true, gameVersions: true, loaders: true }, orderBy: { createdAt: "desc" }, take: 50 },
      },
      orderBy: [{ title: "asc" }],
      take: 100,
    });
    return { projects: projects.map(project => ({ ...project, files: project.files.map(file => ({ ...file, sizeBytes: file.sizeBytes?.toString() ?? null })) })) };
  });
};
