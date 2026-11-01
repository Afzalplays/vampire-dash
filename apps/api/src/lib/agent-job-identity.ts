import { createHash } from "node:crypto";

const stableJson = (value: unknown): string => {
  if (value === null || typeof value !== "object") return JSON.stringify(value) ?? "null";
  if (Array.isArray(value)) return `[${value.map(stableJson).join(",")}]`;
  const record = value as Record<string, unknown>;
  return `{${Object.keys(record).sort().map((key) => `${JSON.stringify(key)}:${stableJson(record[key])}`).join(",")}}`;
};

const goCompatibleJson = (value: unknown): string =>
  stableJson(value)
    .replace(/&/g, "\\u0026")
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");

const goJSONString = (value: string): string =>
  JSON.stringify(value)
    .replace(/&/g, "\\u0026")
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");

export function nodeOperationIdentityHash(job: {
  id: string; nodeId: string; nonce: string;
  operation: { serverId: string; type: string; payload: unknown };
}): string {
  const payload = goCompatibleJson(job.operation.payload);
  const identity = `{"id":${goJSONString(job.id)},"nodeId":${goJSONString(job.nodeId)},"nonce":${goJSONString(job.nonce)},"serverId":${goJSONString(job.operation.serverId)},"action":${goJSONString(job.operation.type)},"payload":${payload}}`;
  return createHash("sha256").update(identity, "utf8").digest("hex");
}

export function nodeAgentTerminalResultHash(result: unknown): string {
  return createHash("sha256").update(goCompatibleJson(result), "utf8").digest("hex");
}
