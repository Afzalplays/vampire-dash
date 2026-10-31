import type { FastifyPluginAsync } from "fastify";
import { prisma } from "../lib/prisma.js";

export const healthRoutes: FastifyPluginAsync = async (app) => {
  app.get("/", async () => ({
    status: "ok",
    service: "vampire-dash-api",
    timestamp: new Date().toISOString()
  }));

  app.get("/ready", async (_request, reply) => {
    try {
      await prisma.$queryRaw\`SELECT 1\`;
      return { status: "ready" };
    } catch {
      return reply.code(503).send({ status: "not_ready" });
    }
  });
};
