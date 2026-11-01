import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { calculateAgentJobRetryDelayMs } from "./agent-job-recovery.js";

describe("agent job retry backoff", () => {
  it("doubles from 30 seconds", () => {
    assert.equal(calculateAgentJobRetryDelayMs(1), 30_000);
    assert.equal(calculateAgentJobRetryDelayMs(2), 60_000);
    assert.equal(calculateAgentJobRetryDelayMs(3), 120_000);
  });
  it("caps at 15 minutes", () => {
    assert.equal(calculateAgentJobRetryDelayMs(20), 900_000);
  });
  it("rejects invalid attempts", () => {
    assert.throws(() => calculateAgentJobRetryDelayMs(0));
  });
});
