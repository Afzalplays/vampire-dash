import type { FastifyPluginAsync } from "fastify";
import { authenticate } from "../lib/authenticate.js";
import { prisma } from "../lib/prisma.js";

export const planRoutes: FastifyPluginAsync = async (app) => {
  app.get("/", async () => {
    const plans = await prisma.plan.findMany({
      where: { enabled: true },
      select: {
        id: true, slug: true, name: true, description: true,
        pricePaise: true, currency: true, billingIntervalDays: true,
        memoryMb: true, diskMb: true, cpuPercent: true, serverSlots: true,
      },
      orderBy: [{ pricePaise: "asc" }, { name: "asc" }],
    });
    return { plans: plans.map((plan) => ({ ...plan, pricePaise: plan.pricePaise.toString() })) };
  });

  app.get("/me/entitlement", async (request, reply) => {
    const user = await authenticate(request, reply);
    if (!user) return;
    const now = new Date();
    const entitlements = await prisma.userPlanEntitlement.findMany({
      where: { userId: user.id, status: "ACTIVE", startsAt: { lte: now }, expiresAt: { gt: now }, plan: { enabled: true } },
      include: { plan: { select: { id: true, slug: true, name: true, memoryMb: true, diskMb: true, cpuPercent: true, serverSlots: true, billingIntervalDays: true } } },
      orderBy: { expiresAt: "desc" },
    });
    return { entitlements };
  });
};
