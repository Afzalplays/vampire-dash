import { env } from "../env.js";
import { prisma } from "../lib/prisma.js";
import { fetchNodeMetrics } from "../lib/node-agent-client.js";

const LOCK_NAMESPACE = 731_204;
const LOCK_ID = 1;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

async function pollNodes(): Promise<void> {
  await prisma.$transaction(async (tx) => {
    const lock = await tx.$queryRaw<Array<{ locked: boolean }>>`SELECT pg_try_advisory_xact_lock(${LOCK_NAMESPACE}, ${LOCK_ID}) AS locked`;
    if (!lock[0]?.locked) {
      console.info("[node-health] another worker owns the polling lease; skipping this cycle");
      return;
    }

    const nodes = await tx.minecraftNode.findMany({
      where: { tokenCiphertext: { not: null }, tokenNonce: { not: null }, tokenAuthTag: { not: null } },
      select: { id: true, name: true, endpoint: true, healthStatus: true, tokenCiphertext: true, tokenNonce: true, tokenAuthTag: true },
      orderBy: { createdAt: "asc" },
    });

    for (const node of nodes) {
      try {
        const metrics = await fetchNodeMetrics(node.endpoint, {
          ciphertext: node.tokenCiphertext!,
          nonce: node.tokenNonce!,
          authTag: node.tokenAuthTag!,
        });
        if (metrics.nodeId !== node.id) throw new Error("Agent identity does not match the registered node");

        await tx.minecraftNode.update({ where: { id: node.id }, data: {
          healthStatus: "ONLINE", lastSeenAt: new Date(), agentVersion: metrics.version,
          cpuCores: metrics.cpuCores, memoryAvailableMb: BigInt(metrics.memoryAvailableMb),
          diskAvailableMb: BigInt(metrics.diskAvailableMb), metricsSampledAt: new Date(metrics.sampledAt),
          lastHealthError: null,
        } });

        if (node.healthStatus !== "ONLINE") {
          await tx.auditLog.create({ data: {
            action: "NODE_HEALTH_RECOVERED", targetId: node.id,
            details: `Node ${node.name} recovered; agent ${metrics.version} responded successfully`,
          } });
        }
      } catch (error) {
        const detail = error instanceof Error ? error.message.slice(0, 300) : "Node health check failed";
        await tx.minecraftNode.update({ where: { id: node.id }, data: { healthStatus: "OFFLINE", lastHealthError: detail } });
        if (node.healthStatus !== "OFFLINE") {
          await tx.auditLog.create({ data: {
            action: "NODE_HEALTH_OFFLINE", targetId: node.id,
            details: `Node ${node.name} became offline: ${detail}`,
          } });
        }
        console.warn(`[node-health] ${node.id} check failed: ${detail}`);
      }
    }
  }, { maxWait: 10_000, timeout: 120_000 });
}

async function main(): Promise<void> {
  console.info(`[node-health] worker started; interval=${env.NODE_HEALTH_INTERVAL_MS}ms`);
  let stopping = false;
  process.once("SIGINT", () => { stopping = true; });
  process.once("SIGTERM", () => { stopping = true; });
  while (!stopping) {
    const startedAt = Date.now();
    try { await pollNodes(); }
    catch (error) { console.error("[node-health] polling cycle failed", error); }
    const remaining = Math.max(0, env.NODE_HEALTH_INTERVAL_MS - (Date.now() - startedAt));
    if (!stopping) await sleep(remaining);
  }
  await prisma.$disconnect();
  console.info("[node-health] worker stopped");
}

main().catch(async (error) => {
  console.error("[node-health] fatal worker error", error);
  await prisma.$disconnect();
  process.exitCode = 1;
});
