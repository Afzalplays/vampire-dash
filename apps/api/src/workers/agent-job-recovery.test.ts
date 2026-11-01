import { describe, expect, it, vi } from "vitest";

vi.mock("../lib/prisma.js", () => ({
  prisma: { $transaction: vi.fn() },
}));

import { prisma } from "../lib/prisma.js";
import { recoverExpiredAgentJobs } from "./agent-job-recovery.js";

describe("recoverExpiredAgentJobs", () => {
  it("does not mutate jobs when another worker owns the advisory lock", async () => {
    vi.mocked(prisma.$transaction).mockImplementationOnce(async (callback: any) => {
      const tx = { $queryRaw: vi.fn().mockResolvedValue([{ locked: false }]) };
      return callback(tx);
    });
    await expect(recoverExpiredAgentJobs()).resolves.toEqual({ retried: 0, dead: 0 });
  });

  it("serializes recovery behind a database advisory lock", async () => {
    const tx = { $queryRaw: vi.fn().mockResolvedValue([{ locked: false }]) };
    vi.mocked(prisma.$transaction).mockImplementationOnce(async (callback: any) => callback(tx));
    await recoverExpiredAgentJobs();
    expect(tx.$queryRaw).toHaveBeenCalledTimes(1);
  });
});
