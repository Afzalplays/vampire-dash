import { prisma } from "../lib/prisma.js";
import { queueEmailNotification } from "../notifications/email.js";

const HOUR = 60 * 60 * 1000;
let running = false;

async function scanExpiringServers() {
  if (running) return;
  running = true;
  try {
    const now = new Date();
    const horizon = new Date(now.getTime() + 7 * 24 * HOUR);
    const servers = await prisma.minecraftServer.findMany({
      where: { expiresAt: { gt: now, lte: horizon }, status: { not: "SUSPENDED" } },
      select: { id: true, name: true, ownerId: true, expiresAt: true },
      take: 500,
    });
    for (const server of servers) {
      if (!server.expiresAt) continue;
      const remaining = server.expiresAt.getTime() - now.getTime();
      const event = remaining <= 24 * HOUR ? "SERVER_EXPIRING_1_DAY" : "SERVER_EXPIRING_7_DAYS";
      try {
        await queueEmailNotification(server.ownerId, event, { serverName: server.name }, `expiry:${server.id}:${event}`);
      } catch (error) {
        console.error("Expiry notification delivery failed", { serverId: server.id, event, error });
      }
    }
  } catch (error) {
    console.error("Expiry notification scan failed", error);
  } finally {
    running = false;
  }
}

console.info("Vampire Dash expiry notification worker started");
await scanExpiringServers();
setInterval(() => void scanExpiringServers(), HOUR);
