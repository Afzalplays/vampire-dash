import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { calculateAgentJobRetryDelayMs } from "./agent-job-retry.js";
import { nodeAgentTerminalResultHash, nodeOperationIdentityHash } from "../lib/agent-job-identity.js";

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

describe("node-agent reconciliation hashes", () => {
  it("matches the Go agent immutable operation identity encoding", () => {
    const hash = nodeOperationIdentityHash({
      id: "job_identifier_1234",
      nodeId: "node-test",
      nonce: "0123456789abcdef0123456789abcdef",
      operation: {
        type: "UPDATE_PROPERTIES",
        payload: { motd: "hello<&", maxPlayers: 10 },
      },
    });
    assert.equal(hash, "8d68e61ff0fe5b7c25542d4ac8c1fd552ae9451fa4dd2f67f81ce3506b8fe2bd");
  });

  it("matches the Go agent terminal result hash encoding", () => {
    const hash = nodeAgentTerminalResultHash({
      status: "SUCCEEDED",
      result: { motd: "hello<&" },
    });
    assert.equal(hash, "da776a42514be63318a00037a51914df73355730d11a54bbb9dbb55a5e2a352e");
  });
});
