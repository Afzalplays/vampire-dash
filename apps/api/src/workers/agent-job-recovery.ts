import { prisma } from "../lib/prisma.js";

const INTERVAL_MS = 15_000;
const LOCK_NAMESPACE = 731_205;
const LOCK_ID = 1;
import { calculateAgentJobRetryDelayMs } from "./agent-job-retry.js";
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Recover abandoned leases. This worker never sends work to an agent and never
 * executes a server operation; it only makes expired leases retryable or DEAD.
 */
export async function recoverExpiredAgentJobs(now = new Date()): Promise<{ retried: number; dead: number }> {
  return prisma.$transaction(async (tx) => {
    const lock = await tx.$queryRaw<Array<{ locked: boolean }>>`SELECT pg_try_advisory_xact_lock(${LOCK_NAMESPACE}, ${LOCK_ID}) AS locked`;
    if (!lock[0]?.locked) return { retried: 0, dead: 0 };

    const expired = await tx.nodeAgentJob.findMany({
      where: { status: { in: ["LEASED", "ACKNOWLEDGED"] }, leaseExpiresAt: { lte: now } },
      select: { id: true, operationId: true, attempts: true, maxAttempts: true, status: true, operation: { select: { serverId: true, type: true } } },
      orderBy: { leaseExpiresAt: "asc" },
      take: 250,
    });

    let retried = 0;
    let dead = 0;
    for (const job of expired) {
      // Once acknowledged, the agent may already have started side effects.
      // Never replay that job after lease loss; fail it closed to avoid duplicates.
      if (job.status === "ACKNOWLEDGED" || job.attempts >= job.maxAttempts) {
        const changed = await tx.nodeAgentJob.updateMany({
          where: { id: job.id, status: job.status, leaseExpiresAt: { lte: now } },
          data: {
            status: "DEAD",
            leaseTokenHash: null,
            leaseExpiresAt: null,
            lastErrorCode: job.status === "ACKNOWLEDGED" ? "ACKNOWLEDGED_LEASE_EXPIRED" : "LEASE_EXPIRED_MAX_ATTEMPTS",
            lastError: job.status === "ACKNOWLEDGED" ? "Acknowledged job lost its agent lease; automatic replay was suppressed to prevent duplicate side effects" : "Agent job lease expired after the maximum number of attempts",
          },
        });
        if (changed.count) {
          dead += 1;
          await tx.minecraftServerOperation.updateMany({
            where: { id: job.operationId, status: { in: ["QUEUED", "RUNNING"] } },
            data: { status: "FAILED", errorCode: job.status === "ACKNOWLEDGED" ? "AGENT_JOB_LEASE_LOST" : "AGENT_JOB_EXHAUSTED", errorMessage: job.status === "ACKNOWLEDGED" ? "Acknowledged operation lost its agent lease; automatic replay was suppressed" : "Node agent did not acknowledge the job before its lease expired", finishedAt: now },
          });
          // Keep the customer lifecycle in sync with terminal job recovery.
          // An acknowledged lease may have performed a side effect before the
          // agent disappeared, so mark the server UNKNOWN rather than guessing.
          if (job.operation.type === "PROVISION_SERVER") {
            await tx.minecraftServer.updateMany({
              where: { id: job.operation.serverId },
              data: { status: job.status === "ACKNOWLEDGED" ? "UNKNOWN" : "ERROR" },
            });
          } else if (job.status === "ACKNOWLEDGED") {
            await tx.minecraftServer.updateMany({
              where: { id: job.operation.serverId },
              data: { status: "UNKNOWN" },
            });
          }
        }
      } else {
        const delay = calculateAgentJobRetryDelayMs(Math.max(1, job.attempts));
        const changed = await tx.nodeAgentJob.updateMany({
          where: { id: job.id, status: "LEASED", leaseExpiresAt: { lte: now } },
          data: {
            status: "PENDING",
            availableAt: new Date(now.getTime() + delay),
            leaseTokenHash: null,
            leaseExpiresAt: null,
            lastErrorCode: "LEASE_EXPIRED",
            lastError: "Agent job lease expired without acknowledgement",
          },
        });
        if (changed.count) retried += 1;
      }
    }
    return { retried, dead };
  }, { maxWait: 10_000, timeout: 30_000 });
}

async function main(): Promise<void> {
  console.info(`[agent-job-recovery] worker started; interval=${INTERVAL_MS}ms`);
  let stopping = false;
  process.once("SIGINT", () => { stopping = true; });
  process.once("SIGTERM", () => { stopping = true; });
  while (!stopping) {
    const startedAt = Date.now();
    try {
      const result = await recoverExpiredAgentJobs();
      if (result.retried || result.dead) console.info("[agent-job-recovery] recovered expired leases", result);
    } catch (error) {
      console.error("[agent-job-recovery] cycle failed", error);
    }
    if (!stopping) await sleep(Math.max(0, INTERVAL_MS - (Date.now() - startedAt)));
  }
  await prisma.$disconnect();
  console.info("[agent-job-recovery] worker stopped");
}

main().catch(async (error) => {
  console.error("[agent-job-recovery] fatal worker error", error);
  await prisma.$disconnect();
  process.exitCode = 1;
});
