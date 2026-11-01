import type { FastifyPluginAsync } from "fastify";
import { createHmac, randomBytes } from "node:crypto";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { env } from "../env.js";
import { requireAdmin } from "../lib/authenticate.js";
import { listPaymentProviders } from "../billing/registry.js";
import { encryptNodeCredential } from "../lib/node-credentials.js";
import { fetchNodeMetrics } from "../lib/node-agent-client.js";

const nodeCreateSchema = z.object({
  name: z.string().trim().min(2).max(64),
  location: z.string().trim().min(2).max(64),
  endpoint: z.string().url().max(255).refine((value) => {
    try { const url = new URL(value); return url.protocol === "https:" && !url.username && !url.password && !url.search && !url.hash; }
    catch { return false; }
  }, "Node endpoint must be an HTTPS URL without credentials, query, or fragment"),
  memoryMb: z.number().int().positive().max(4_194_304),
  diskMb: z.number().int().positive().max(16_777_216),
});
const nodeTokenDigest = (token: string) => createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");

export const adminRoutes: FastifyPluginAsync = async (app) => {
  app.post("/software/releases", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const body = z.object({
      software: z.enum(["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"]),
      minecraftVersion: z.string().trim().min(1).max(32),
      softwareVersion: z.string().trim().min(1).max(64),
      javaMajor: z.number().int().min(8).max(30),
      dockerImage: z.string().trim().min(3).max(255).regex(/^[A-Za-z0-9._/:@-]+$/, "Invalid container image reference"),
      downloadUrl: z.string().url().max(2048).optional(),
      checksumSha256: z.string().regex(/^[a-fA-F0-9]{64}$/).optional(),
      recommended: z.boolean().optional(),
      enabled: z.boolean().optional(),
    }).safeParse(request.body);
    if (!body.success) return reply.code(400).send({ error: "Invalid software release", details: body.error.flatten() });
    try {
      const release = await prisma.minecraftSoftwareRelease.create({ data: body.data });
      await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "SOFTWARE_RELEASE_CREATED", targetId: release.id, details: `Added ${release.software} ${release.softwareVersion} for Minecraft ${release.minecraftVersion}; enabled=${release.enabled}` } });
      return reply.code(201).send({ release });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "This software release already exists" });
      throw error;
    }
  });
  app.get("/software/releases", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const releases = await prisma.minecraftSoftwareRelease.findMany({
      select: { id: true, software: true, minecraftVersion: true, softwareVersion: true, javaMajor: true, dockerImage: true, downloadUrl: true, checksumSha256: true, recommended: true, enabled: true, createdAt: true, updatedAt: true },
      orderBy: [{ enabled: "desc" }, { software: "asc" }, { minecraftVersion: "desc" }, { softwareVersion: "desc" }],
      take: 500,
    });
    return { releases };
  });
  app.patch("/software/releases/:releaseId", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ releaseId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = z.object({ enabled: z.boolean().optional(), recommended: z.boolean().optional() }).strict()
      .refine((value) => value.enabled !== undefined || value.recommended !== undefined, "At least one field is required")
      .safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid software release update" });
    const existing = await prisma.minecraftSoftwareRelease.findUnique({ where: { id: params.data.releaseId } });
    if (!existing) return reply.code(404).send({ error: "Software release not found" });
    const release = await prisma.minecraftSoftwareRelease.update({ where: { id: existing.id }, data: body.data });
    await prisma.auditLog.create({ data: {
      actorUserId: admin.id, action: "SOFTWARE_RELEASE_UPDATED", targetId: release.id,
      details: `Updated ${release.software} ${release.softwareVersion} for Minecraft ${release.minecraftVersion}; enabled=${release.enabled}; recommended=${release.recommended}`,
    } });
    return { release };
  });

  app.post("/artifacts/projects", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const parsed = z.object({
      kind: z.enum(["PLUGIN", "MOD", "MODPACK"]),
      provider: z.enum(["MODRINTH", "CURSEFORGE"]),
      providerProjectId: z.string().trim().min(1).max(128),
      slug: z.string().trim().min(1).max(128).regex(/^[a-zA-Z0-9_-]+$/),
      title: z.string().trim().min(1).max(160),
      summary: z.string().max(2000).default(""),
      projectUrl: z.string().url().max(2048),
      sourceUrl: z.string().url().max(2048).optional(),
      licenseId: z.string().trim().max(128).optional(),
      supportedSoftware: z.array(z.enum(["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"])).max(9).default([]),
      supportedGameVersions: z.array(z.string().trim().min(1).max(32)).max(100).default([]),
      loaders: z.array(z.string().trim().min(1).max(32)).max(20).default([]),
      enabled: z.boolean().default(false),
    }).safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid artifact project", details: parsed.error.flatten() });
    const allowedHosts = parsed.data.provider === "MODRINTH" ? ["modrinth.com", "www.modrinth.com"] : ["curseforge.com", "www.curseforge.com"];
    const validProviderUrl = (value: string) => { try { const url = new URL(value); return url.protocol === "https:" && allowedHosts.includes(url.hostname) && !url.username && !url.password; } catch { return false; } };
    if (!validProviderUrl(parsed.data.projectUrl) || (parsed.data.sourceUrl && !validProviderUrl(parsed.data.sourceUrl))) return reply.code(400).send({ error: "Project and source URLs must use the selected provider's HTTPS domain" });
    try {
      const project = await prisma.minecraftArtifactProject.create({ data: parsed.data });
      await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "ARTIFACT_PROJECT_CREATED", targetId: project.id, details: `Added ${project.kind} ${project.title} from ${project.provider}; enabled=${project.enabled}` } });
      return reply.code(201).send({ project });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "This provider project already exists" });
      throw error;
    }
  });
  app.post("/artifacts/projects/:projectId/files", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ projectId: z.string().min(1).max(128) }).safeParse(request.params);
    const parsed = z.object({
      providerFileId: z.string().trim().min(1).max(128),
      versionNumber: z.string().trim().min(1).max(64),
      downloadUrl: z.string().url().max(2048),
      sha256: z.string().regex(/^[a-fA-F0-9]{64}$/).optional(),
      sizeBytes: z.number().int().positive().max(2_147_483_647).optional(),
      gameVersions: z.array(z.string().trim().min(1).max(32)).max(100).default([]),
      loaders: z.array(z.string().trim().min(1).max(32)).max(20).default([]),
      enabled: z.boolean().default(false),
    }).safeParse(request.body);
    if (!params.success || !parsed.success) return reply.code(400).send({ error: "Invalid artifact file", details: parsed.success ? undefined : parsed.error.flatten() });
    const project = await prisma.minecraftArtifactProject.findUnique({ where: { id: params.data.projectId } });
    if (!project) return reply.code(404).send({ error: "Artifact project not found" });
    const downloadHostAllowed = (value: string) => { try { const url = new URL(value); return url.protocol === "https:" && (project.provider === "MODRINTH" ? url.hostname === "cdn.modrinth.com" : url.hostname.endsWith(".forgecdn.net") || url.hostname === "mediafilez.forgecdn.net"); } catch { return false; } };
    if (!downloadHostAllowed(parsed.data.downloadUrl)) return reply.code(400).send({ error: "Download URL host is not allowed for this provider" });
    try {
      const file = await prisma.minecraftArtifactFile.create({ data: { ...parsed.data, projectId: project.id, sizeBytes: parsed.data.sizeBytes === undefined ? undefined : BigInt(parsed.data.sizeBytes) } });
      await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "ARTIFACT_FILE_CREATED", targetId: file.id, details: `Added ${project.kind} file ${file.versionNumber} for ${project.title}; enabled=${file.enabled}` } });
      return reply.code(201).send({ file: { ...file, sizeBytes: file.sizeBytes?.toString() ?? null } });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "This provider file already exists for the project" });
      throw error;
    }
  });
  app.get("/artifacts/projects", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const projects = await prisma.minecraftArtifactProject.findMany({
      select: {
        id: true, kind: true, provider: true, providerProjectId: true, slug: true, title: true, summary: true,
        projectUrl: true, sourceUrl: true, licenseId: true, supportedSoftware: true, supportedGameVersions: true,
        loaders: true, enabled: true, createdAt: true, updatedAt: true,
        files: {
          select: { id: true, providerFileId: true, versionNumber: true, downloadUrl: true, sha256: true, sizeBytes: true, gameVersions: true, loaders: true, enabled: true, createdAt: true, updatedAt: true },
          orderBy: { createdAt: "desc" },
        },
      },
      orderBy: [{ enabled: "desc" }, { title: "asc" }],
      take: 500,
    });
    return { projects: projects.map(project => ({
      ...project,
      files: project.files.map(file => ({ ...file, sizeBytes: file.sizeBytes?.toString() ?? null })),
    })) };
  });
  app.patch("/artifacts/projects/:projectId", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ projectId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = z.object({ enabled: z.boolean() }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid artifact project update" });
    const existing = await prisma.minecraftArtifactProject.findUnique({ where: { id: params.data.projectId }, select: { id: true, title: true, enabled: true } });
    if (!existing) return reply.code(404).send({ error: "Artifact project not found" });
    const project = await prisma.minecraftArtifactProject.update({ where: { id: existing.id }, data: body.data });
    await prisma.auditLog.create({ data: {
      actorUserId: admin.id, action: "ARTIFACT_PROJECT_UPDATED", targetId: project.id,
      details: `Updated artifact project ${project.title}; enabled=${project.enabled}`,
    } });
    return { project };
  });
  app.patch("/artifacts/files/:fileId", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ fileId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = z.object({ enabled: z.boolean() }).strict().safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid artifact file update" });
    const existing = await prisma.minecraftArtifactFile.findUnique({
      where: { id: params.data.fileId },
      select: { id: true, versionNumber: true, project: { select: { title: true } } },
    });
    if (!existing) return reply.code(404).send({ error: "Artifact file not found" });
    const file = await prisma.minecraftArtifactFile.update({ where: { id: existing.id }, data: body.data });
    await prisma.auditLog.create({ data: {
      actorUserId: admin.id, action: "ARTIFACT_FILE_UPDATED", targetId: file.id,
      details: `Updated artifact file ${existing.versionNumber} for ${existing.project.title}; enabled=${file.enabled}`,
    } });
    return { file: { ...file, sizeBytes: file.sizeBytes?.toString() ?? null } };
  });
  app.get("/users", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const users = await prisma.user.findMany({ select: { id: true, email: true, username: true, role: true, status: true, createdAt: true }, orderBy: { createdAt: "desc" }, take: 100 });
    return { users };
  });
  app.get("/payment-providers", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    return { defaultProvider: "famgateway", availableProviders: listPaymentProviders() };
  });
  app.get("/nodes", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const nodes = await prisma.minecraftNode.findMany({
      select: { id: true, name: true, location: true, endpoint: true, enabled: true, memoryMb: true, diskMb: true, reservedMemoryMb: true, reservedDiskMb: true, memoryOverheadMb: true, diskOverheadMb: true, healthStatus: true, lastSeenAt: true, agentVersion: true, cpuCores: true, memoryAvailableMb: true, diskAvailableMb: true, metricsSampledAt: true, lastHealthError: true, createdAt: true, _count: { select: { servers: true, reservations: true } } },
      orderBy: { createdAt: "desc" },
    });
    return { nodes };
  });
  app.post("/nodes", { config: { rateLimit: { max: 5, timeWindow: "1 hour" } } }, async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const parsed = nodeCreateSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid node details", details: parsed.error.flatten() });
    if (!env.NODE_CREDENTIALS_KEY) return reply.code(503).send({ error: "Node credential encryption is not configured; set NODE_CREDENTIALS_KEY before registering nodes" });
    const endpoint = new URL(parsed.data.endpoint).toString().replace(/\/$/, "");
    const duplicate = await prisma.minecraftNode.findFirst({ where: { endpoint }, select: { id: true } });
    if (duplicate) return reply.code(409).send({ error: "A node with this endpoint is already registered" });
    const token = randomBytes(32).toString("base64url");
    const encrypted = encryptNodeCredential(token);
    const node = await prisma.minecraftNode.create({ data: {
      ...parsed.data, endpoint, tokenHash: nodeTokenDigest(token),
      tokenCiphertext: encrypted.ciphertext, tokenNonce: encrypted.nonce, tokenAuthTag: encrypted.authTag, enabled: false,
    }, select: { id: true, name: true, location: true, endpoint: true, enabled: true, memoryMb: true, diskMb: true, createdAt: true } });
    await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_REGISTERED", targetId: node.id, details: `Registered node ${node.name} at ${node.endpoint}; disabled pending secure agent enrollment` } });
    return reply.code(201).send({ node, agentToken: token, warning: "Copy this token now. It will not be shown again. Node remains disabled until mTLS enrollment is implemented." });
  });
  app.post("/nodes/:nodeId/check", { config: { rateLimit: { max: 12, timeWindow: "1 minute" } } }, async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ nodeId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid node ID" });
    const node = await prisma.minecraftNode.findUnique({ where: { id: params.data.nodeId } });
    if (!node) return reply.code(404).send({ error: "Node not found" });
    if (!node.tokenCiphertext || !node.tokenNonce || !node.tokenAuthTag) return reply.code(409).send({ error: "Node credentials are unavailable; re-register this node" });
    try {
      const metrics = await fetchNodeMetrics(node.endpoint, { ciphertext: node.tokenCiphertext, nonce: node.tokenNonce, authTag: node.tokenAuthTag });
      if (metrics.nodeId !== node.id) throw new Error("Agent identity does not match the registered node");
      const updated = await prisma.minecraftNode.update({ where: { id: node.id }, data: {
        healthStatus: "ONLINE", lastSeenAt: new Date(), agentVersion: metrics.version, cpuCores: metrics.cpuCores,
        memoryAvailableMb: BigInt(metrics.memoryAvailableMb), diskAvailableMb: BigInt(metrics.diskAvailableMb),
        metricsSampledAt: new Date(metrics.sampledAt), lastHealthError: null,
      }, select: { id: true, name: true, healthStatus: true, lastSeenAt: true, agentVersion: true, cpuCores: true, memoryAvailableMb: true, diskAvailableMb: true, metricsSampledAt: true } });
      await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_HEALTH_CHECKED", targetId: node.id, details: `Node agent ${metrics.version} responded successfully` } });
      return { node: { ...updated, memoryAvailableMb: updated.memoryAvailableMb?.toString() ?? null, diskAvailableMb: updated.diskAvailableMb?.toString() ?? null } };
    } catch (error) {
      const message = error instanceof Error ? error.message.slice(0, 300) : "Node health check failed";
      await prisma.minecraftNode.update({ where: { id: node.id }, data: { healthStatus: "OFFLINE", lastHealthError: message } });
      await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_HEALTH_CHECK_FAILED", targetId: node.id, details: message } });
      return reply.code(502).send({ error: "Node health check failed", detail: message });
    }
  });
  app.post("/nodes/:nodeId/reservations", { config: { rateLimit: { max: 30, timeWindow: "1 minute" } } }, async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ nodeId: z.string().min(1).max(128) }).safeParse(request.params);
    const body = z.object({
      memoryMb: z.number().int().positive().max(4_194_304),
      diskMb: z.number().int().positive().max(16_777_216),
      idempotencyKey: z.string().min(16).max(128).regex(/^[A-Za-z0-9._:-]+$/),
    }).safeParse(request.body);
    if (!params.success || !body.success) return reply.code(400).send({ error: "Invalid capacity reservation request" });
    const existing = await prisma.nodeCapacityReservation.findUnique({ where: { idempotencyKey: body.data.idempotencyKey } });
    if (existing) {
      if (existing.nodeId !== params.data.nodeId || existing.memoryMb !== body.data.memoryMb || existing.diskMb !== body.data.diskMb) {
        return reply.code(409).send({ error: "Idempotency key was already used for a different reservation" });
      }
      return { reservation: existing, reused: true };
    }
    try {
      const reservation = await prisma.$transaction(async (tx) => {
        await tx.$queryRaw`SELECT "id" FROM "MinecraftNode" WHERE "id" = ${params.data.nodeId} FOR UPDATE`;
        const node = await tx.minecraftNode.findUnique({ where: { id: params.data.nodeId } });
        if (!node) throw new Error("NODE_NOT_FOUND");
        if (!node.enabled || node.healthStatus !== "ONLINE") throw new Error("NODE_NOT_READY");
        await tx.nodeCapacityReservation.updateMany({
          where: { nodeId: node.id, status: "ACTIVE", expiresAt: { lte: new Date() } },
          data: { status: "EXPIRED" },
        });
        const used = await tx.nodeCapacityReservation.aggregate({
          where: { nodeId: node.id, status: "ACTIVE", expiresAt: { gt: new Date() } },
          _sum: { memoryMb: true, diskMb: true },
        });
        const memoryUsed = used._sum.memoryMb ?? 0;
        const diskUsed = used._sum.diskMb ?? 0;
        if (memoryUsed + body.data.memoryMb > node.memoryMb - node.memoryOverheadMb ||
            diskUsed + body.data.diskMb > node.diskMb - node.diskOverheadMb) throw new Error("CAPACITY_EXCEEDED");
        const created = await tx.nodeCapacityReservation.create({ data: {
          nodeId: node.id, memoryMb: body.data.memoryMb, diskMb: body.data.diskMb,
          idempotencyKey: body.data.idempotencyKey, expiresAt: new Date(Date.now() + 15 * 60 * 1000),
        } });
        const totals = await tx.nodeCapacityReservation.aggregate({
          where: { nodeId: node.id, status: "ACTIVE", expiresAt: { gt: new Date() } },
          _sum: { memoryMb: true, diskMb: true },
        });
        await tx.minecraftNode.update({ where: { id: node.id }, data: {
          reservedMemoryMb: totals._sum.memoryMb ?? 0,
          reservedDiskMb: totals._sum.diskMb ?? 0,
        } });
        await tx.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_CAPACITY_RESERVED", targetId: created.id, details: `Reserved ${created.memoryMb} MiB RAM and ${created.diskMb} MiB disk on node ${node.id}; expires in 15 minutes` } });
        return created;
      }, { isolationLevel: "Serializable", maxWait: 10000, timeout: 15000 });
      return reply.code(201).send({ reservation, reused: false });
    } catch (error) {
      const code = error instanceof Error ? error.message : "";
      if (code === "NODE_NOT_FOUND") return reply.code(404).send({ error: "Node not found" });
      if (code === "NODE_NOT_READY") return reply.code(409).send({ error: "Node must be enabled and online before capacity can be reserved" });
      if (code === "CAPACITY_EXCEEDED") return reply.code(409).send({ error: "Insufficient allocatable node capacity" });
      if (code.includes("P2034")) return reply.code(409).send({ error: "Concurrent capacity update; retry with the same idempotency key" });
      throw error;
    }
  });
  app.post("/nodes/:nodeId/reservations/:reservationId/release", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ nodeId: z.string().min(1), reservationId: z.string().min(1) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid reservation reference" });
    const released = await prisma.$transaction(async (tx) => {
      await tx.$queryRaw`SELECT "id" FROM "MinecraftNode" WHERE "id" = ${params.data.nodeId} FOR UPDATE`;
      const changed = await tx.nodeCapacityReservation.updateMany({
        where: { id: params.data.reservationId, nodeId: params.data.nodeId, status: "ACTIVE" },
        data: { status: "RELEASED" },
      });
      if (!changed.count) return false;
      const now = new Date();
      await tx.nodeCapacityReservation.updateMany({
        where: { nodeId: params.data.nodeId, status: "ACTIVE", expiresAt: { lte: now } },
        data: { status: "EXPIRED" },
      });
      const totals = await tx.nodeCapacityReservation.aggregate({
        where: { nodeId: params.data.nodeId, status: "ACTIVE", expiresAt: { gt: now } },
        _sum: { memoryMb: true, diskMb: true },
      });
      await tx.minecraftNode.update({ where: { id: params.data.nodeId }, data: {
        reservedMemoryMb: totals._sum.memoryMb ?? 0,
        reservedDiskMb: totals._sum.diskMb ?? 0,
      } });
      await tx.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_CAPACITY_RELEASED", targetId: params.data.reservationId, details: `Released capacity reservation ${params.data.reservationId}` } });
      return true;
    }, { isolationLevel: "Serializable", maxWait: 10000, timeout: 15000 });
    if (!released) return reply.code(404).send({ error: "Active reservation not found" });
    return { ok: true, status: "RELEASED" };
  });
  app.post("/nodes/:nodeId/disable", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
    const params = z.object({ nodeId: z.string().min(1).max(128) }).safeParse(request.params);
    if (!params.success) return reply.code(400).send({ error: "Invalid node ID" });
    const node = await prisma.minecraftNode.findUnique({ where: { id: params.data.nodeId }, select: { id: true, name: true, enabled: true } });
    if (!node) return reply.code(404).send({ error: "Node not found" });
    await prisma.minecraftNode.update({ where: { id: node.id }, data: { enabled: false } });
    await prisma.auditLog.create({ data: { actorUserId: admin.id, action: "NODE_DISABLED", targetId: node.id, details: `Disabled node ${node.name}` } });
    return { ok: true, enabled: false };
  });
  app.post("/users/:userId/wallet-adjustment", async (request, reply) => {
    const admin = await requireAdmin(request, reply); if (!admin) return;
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
