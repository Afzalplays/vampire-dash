import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { prisma } from "../lib/prisma.js";
import { requireAdmin } from "../lib/authenticate.js";

const planFields = {
  slug: z.string().trim().min(2).max(64).regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/),
  name: z.string().trim().min(2).max(100),
  description: z.string().trim().max(1000).default(""),
  pricePaise: z.number().int().safe().min(0).max(100_000_000),
  currency: z.string().trim().length(3).regex(/^[A-Z]{3}$/).default("INR"),
  billingIntervalDays: z.number().int().min(1).max(366),
  memoryMb: z.number().int().min(256).max(4_194_304),
  diskMb: z.number().int().min(1024).max(16_777_216),
  cpuPercent: z.number().int().min(10).max(6400),
  serverSlots: z.number().int().min(1).max(100),
  enabled: z.boolean().default(false),
};

export const adminPlanRoutes: FastifyPluginAsync = async (app) => {
  app.get("/plans", async (request, reply) => {
    if (!await requireAdmin(request, reply)) return;
    const plans = await prisma.plan.findMany({
      select: { id: true, slug: true, name: true, description: true, pricePaise: true, currency: true, billingIntervalDays: true, memoryMb: true, diskMb: true, cpuPercent: true, serverSlots: true, enabled: true, createdAt: true, updatedAt: true },
      orderBy: [{ enabled: "desc" }, { pricePaise: "asc" }, { name: "asc" }],
      take: 500,
    });
    return { plans: plans.map(plan => ({ ...plan, pricePaise: plan.pricePaise.toString() })) };
  });

  app.post("/plans", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const parsed = z.object(planFields).strict().safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Invalid plan", details: parsed.error.flatten() });
    try {
      const plan = await prisma.$transaction(async tx => {
        const created = await tx.plan.create({ data: { ...parsed.data, pricePaise: BigInt(parsed.data.pricePaise) } });
        await tx.auditLog.create({ data: { actorUserId: admin.id, action: "PLAN_CREATED", targetId: created.id, details: `Created plan ${created.slug}; enabled=${created.enabled}; pricePaise=${created.pricePaise}` } });
        return created;
      });
      return reply.code(201).send({ plan: { ...plan, pricePaise: plan.pricePaise.toString() } });
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "Plan slug already exists" });
      throw error;
    }
  });

  app.patch("/plans/:planId", async (request, reply) => {
    const admin = await requireAdmin(request, reply);
    if (!admin) return;
    const params = z.object({ planId: z.string().min(1).max(128) }).safeParse(request.params);
    const patchSchema = z.object({
      slug: planFields.slug.optional(),
      name: planFields.name.optional(),
      description: planFields.description.optional(),
      pricePaise: planFields.pricePaise.optional(),
      currency: planFields.currency.optional(),
      billingIntervalDays: planFields.billingIntervalDays.optional(),
      memoryMb: planFields.memoryMb.optional(),
      diskMb: planFields.diskMb.optional(),
      cpuPercent: planFields.cpuPercent.optional(),
      serverSlots: planFields.serverSlots.optional(),
      enabled: z.boolean().optional(),
    }).strict().refine(value => Object.keys(value).length > 0, "At least one field is required");
    const parsed = patchSchema.safeParse(request.body);
    if (!params.success || !parsed.success) return reply.code(400).send({ error: "Invalid plan update", details: parsed.success ? undefined : parsed.error.flatten() });
    const existing = await prisma.plan.findUnique({ where: { id: params.data.planId }, select: { id: true } });
    if (!existing) return reply.code(404).send({ error: "Plan not found" });
    const data = { ...parsed.data, ...(parsed.data.pricePaise === undefined ? {} : { pricePaise: BigInt(parsed.data.pricePaise) }) };
    try {
      const plan = await prisma.$transaction(async tx => {
        const updated = await tx.plan.update({ where: { id: existing.id }, data });
        await tx.auditLog.create({ data: { actorUserId: admin.id, action: "PLAN_UPDATED", targetId: updated.id, details: `Updated plan ${updated.slug}; enabled=${updated.enabled}` } });
        return updated;
      });
      return { plan: { ...plan, pricePaise: plan.pricePaise.toString() } };
    } catch (error) {
      if (typeof error === "object" && error !== null && "code" in error && error.code === "P2002") return reply.code(409).send({ error: "Plan slug already exists" });
      throw error;
    }
  });
};
