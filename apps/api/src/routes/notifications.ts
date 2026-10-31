import type { FastifyPluginAsync } from "fastify";
import { requireAuth } from "../lib/authenticate.js";
import { prisma } from "../lib/prisma.js";

export const notificationRoutes: FastifyPluginAsync = async (app) => {
  app.get("/preferences", async (request, reply) => {
    const user = await requireAuth(request, reply);
    if (!user) return;
    const preferences = await prisma.notificationPreference.findUnique({ where: { userId: user.id } });
    return { emailEnabled: preferences?.emailEnabled ?? true, serverAlerts: preferences?.serverAlerts ?? true, billingAlerts: preferences?.billingAlerts ?? true, securityAlerts: preferences?.securityAlerts ?? true };
  });
  app.patch("/preferences", async (request, reply) => {
    const user = await requireAuth(request, reply);
    if (!user) return;
    const { z } = await import("zod");
    const parsed = z.object({ emailEnabled: z.boolean().optional(), serverAlerts: z.boolean().optional(), billingAlerts: z.boolean().optional(), securityAlerts: z.boolean().optional() }).safeParse(request.body);
    if (!parsed.success || Object.keys(parsed.data ?? {}).length === 0) return reply.code(400).send({ error: "Provide at least one valid preference" });
    const value = await prisma.notificationPreference.upsert({ where: { userId: user.id }, create: { userId: user.id, ...parsed.data }, update: parsed.data });
    return { emailEnabled: value.emailEnabled, serverAlerts: value.serverAlerts, billingAlerts: value.billingAlerts, securityAlerts: value.securityAlerts };
  });
};
