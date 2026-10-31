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
    let cursor: string | undefined;
    do {
      const servers = await prisma.minecraftServer.findMany({
        where: { expiresAt: { gt: now, lte: horizon }, status: { not: "SUSPENDED" } },
        select: { id: true, name: true, ownerId: true, expiresAt: true },
        orderBy: { id: "asc" },
        take: 500,
        ...(cursor ? { cursor: { id: cursor }, skip: 1 } : {}),
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
      cursor = servers.length === 500 ? servers[servers.length - 1]?.id : undefined;
    } while (cursor);
  } catch (error) {
    console.error("Expiry notification scan failed", error);
  } finally {
    running = false;
  }
}

console.info("Vampire Dash expiry notification worker started");
await scanExpiringServers();
setInterval(() => void scanExpiringServers(), HOUR);
