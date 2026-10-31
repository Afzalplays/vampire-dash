import { prisma } from "../lib/prisma.js";

const INTERVAL_MS = 30_000;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Expire timed-out capacity holds and reconcile the node's cached counters.
 * Every node is locked before changing reservation state or cached totals, so
 * this worker is safe to run in more than one API/worker process.
 */
async function expireReservations(): Promise<void> {
  const now = new Date();
  const candidates = await prisma.nodeCapacityReservation.groupBy({
    by: ["nodeId"],
    where: { status: "ACTIVE", expiresAt: { lte: now } },
  });

  for (const candidate of candidates) {
    await prisma.$transaction(async (tx) => {
      const locked = await tx.$queryRaw<Array<{ id: string }>>`SELECT "id" FROM "MinecraftNode" WHERE "id" = ${candidate.nodeId} FOR UPDATE`;
      if (!locked.length) return;

      const expired = await tx.nodeCapacityReservation.updateMany({
        where: { nodeId: candidate.nodeId, status: "ACTIVE", expiresAt: { lte: now } },
        data: { status: "EXPIRED" },
      });
      const totals = await tx.nodeCapacityReservation.aggregate({
        where: { nodeId: candidate.nodeId, status: "ACTIVE", expiresAt: { gt: now } },
        _sum: { memoryMb: true, diskMb: true },
      });
      await tx.minecraftNode.update({
        where: { id: candidate.nodeId },
        data: {
          reservedMemoryMb: totals._sum.memoryMb ?? 0,
          reservedDiskMb: totals._sum.diskMb ?? 0,
        },
      });
      if (expired.count > 0) {
        await tx.auditLog.create({
          data: {
            action: "NODE_CAPACITY_RESERVATIONS_EXPIRED",
            targetId: candidate.nodeId,
            details: `Expired ${expired.count} capacity reservation(s) and reconciled cached node capacity counters`,
          },
        });
      }
    }, { isolationLevel: "Serializable", maxWait: 10_000, timeout: 15_000 });
  }
}

async function main(): Promise<void> {
  console.info(`[reservation-expiry] worker started; interval=${INTERVAL_MS}ms`);
  let stopping = false;
  process.once("SIGINT", () => { stopping = true; });
  process.once("SIGTERM", () => { stopping = true; });
  while (!stopping) {
    const startedAt = Date.now();
    try { await expireReservations(); }
    catch (error) { console.error("[reservation-expiry] cycle failed", error); }
    if (!stopping) await sleep(Math.max(0, INTERVAL_MS - (Date.now() - startedAt)));
  }
  await prisma.$disconnect();
  console.info("[reservation-expiry] worker stopped");
}

main().catch(async (error) => {
  console.error("[reservation-expiry] fatal worker error", error);
  await prisma.$disconnect();
  process.exitCode = 1;
});
