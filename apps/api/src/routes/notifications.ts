import type { FastifyPluginAsync } from "fastify";
import { z } from "zod";
import { authenticate } from "../lib/authenticate.js";
import { prisma } from "../lib/prisma.js";

const preferenceSchema = z.object({
  emailEnabled: z.boolean().optional(),
  serverAlerts: z.boolean().optional(),
  billingAlerts: z.boolean().optional(),
  securityAlerts: z.boolean().optional(),
}).refine(value => Object.keys(value).length > 0);

export const notificationRoutes: FastifyPluginAsync = async (app) => {
  app.get("/preferences", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const preferences = await prisma.notificationPreference.findUnique({ where: { userId: user.id } });
    return { emailEnabled: preferences?.emailEnabled ?? true, serverAlerts: preferences?.serverAlerts ?? true, billingAlerts: preferences?.billingAlerts ?? true, securityAlerts: preferences?.securityAlerts ?? true };
  });
  app.patch("/preferences", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const parsed = preferenceSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: "Provide at least one valid preference" });
    const value = await prisma.notificationPreference.upsert({ where: { userId: user.id }, create: { userId: user.id, ...parsed.data }, update: parsed.data });
    return { emailEnabled: value.emailEnabled, serverAlerts: value.serverAlerts, billingAlerts: value.billingAlerts, securityAlerts: value.securityAlerts };
  });
};
