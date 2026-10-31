import { createHmac } from "node:crypto";
import type { FastifyReply, FastifyRequest } from "fastify";
import { env } from "../env.js";
import { prisma } from "./prisma.js";

const digest = (token: string) => createHmac("sha256", env.SESSION_SECRET).update(token, "utf8").digest("hex");

export async function authenticate(request: FastifyRequest, reply: FastifyReply) {
  const token = request.cookies.vd_session;
  if (!token) {
    await reply.code(401).send({ error: "Authentication required" });
    return null;
  }
  const session = await prisma.session.findUnique({ where: { tokenHash: digest(token) }, include: { user: true } });
  if (!session || session.expiresAt <= new Date() || session.user.status !== "ACTIVE" || !session.user.emailVerifiedAt) {
    if (session) await prisma.session.delete({ where: { id: session.id } });
    await reply.code(401).send({ error: "Session expired" });
    return null;
  }
  return session.user;
}

export async function requireAdmin(request: FastifyRequest, reply: FastifyReply) {
  const user = await authenticate(request, reply);
  if (!user) return null;
  if (user.role !== "ADMIN") {
    await reply.code(403).send({ error: "Administrator access required" });
    return null;
  }
  return user;
}
