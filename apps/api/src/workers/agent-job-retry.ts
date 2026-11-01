export const MAX_AGENT_JOB_BACKOFF_MS = 15 * 60_000;

export function calculateAgentJobRetryDelayMs(attempts: number): number {
  if (!Number.isInteger(attempts) || attempts < 1) throw new Error("attempts must be a positive integer");
  return Math.min(30_000 * (2 ** Math.max(0, attempts - 1)), MAX_AGENT_JOB_BACKOFF_MS);
}
