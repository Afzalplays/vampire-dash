import { readFile } from "node:fs/promises";
import { request as httpsRequest } from "node:https";
import { URL } from "node:url";
import { env } from "../env.js";
import { decryptNodeCredential } from "./node-credentials.js";

export interface NodeAgentMetrics {
  nodeId: string;
  version: string;
  cpuCores: number;
  memoryTotalMb: number;
  memoryAvailableMb: number;
  diskTotalMb: number;
  diskAvailableMb: number;
  dataDirectory: string;
  sampledAt: string;
}

function getJson<T>(url: URL, token: string, options: { ca: Buffer; cert: Buffer; key: Buffer }): Promise<T> {
  return new Promise((resolve, reject) => {
    const req = httpsRequest(url, {
      method: "GET",
      headers: { authorization: `Bearer ${token}`, accept: "application/json" },
      ca: options.ca,
      cert: options.cert,
      key: options.key,
      rejectUnauthorized: true,
      servername: url.hostname,
      timeout: env.NODE_AGENT_TIMEOUT_MS,
    }, (res) => {
      const chunks: Buffer[] = [];
      let size = 0;
      res.on("data", (chunk: Buffer) => {
        size += chunk.length;
        if (size > 64 * 1024) {
          req.destroy(new Error("Node agent response exceeded 64 KiB"));
          return;
        }
        chunks.push(chunk);
      });
      res.on("end", () => {
        if (res.statusCode !== 200) return reject(new Error(`Node agent returned HTTP ${res.statusCode ?? "unknown"}`));
        try { resolve(JSON.parse(Buffer.concat(chunks).toString("utf8")) as T); }
        catch { reject(new Error("Node agent returned invalid JSON")); }
      });
    });
    req.on("timeout", () => req.destroy(new Error("Node agent request timed out")));
    req.on("error", reject);
    req.end();
  });
}

export async function fetchNodeMetrics(endpoint: string, encrypted: { ciphertext: string; nonce: string; authTag: string }): Promise<NodeAgentMetrics> {
  if (!env.NODE_AGENT_CA_FILE || !env.NODE_AGENT_CLIENT_CERT_FILE || !env.NODE_AGENT_CLIENT_KEY_FILE) {
    throw new Error("Panel mTLS client is not configured");
  }
  const base = new URL(endpoint);
  if (base.protocol !== "https:" || base.username || base.password || base.search || base.hash) {
    throw new Error("Registered node endpoint is not a valid HTTPS origin");
  }
  const [ca, cert, key] = await Promise.all([
    readFile(env.NODE_AGENT_CA_FILE), readFile(env.NODE_AGENT_CLIENT_CERT_FILE), readFile(env.NODE_AGENT_CLIENT_KEY_FILE),
  ]);
  const token = decryptNodeCredential(encrypted);
  const url = new URL("/v1/node/metrics", base);
  const metrics = await getJson<NodeAgentMetrics>(url, token, { ca, cert, key });
  if (!metrics || typeof metrics.nodeId !== "string" || typeof metrics.version !== "string" ||
      !Number.isSafeInteger(metrics.cpuCores) || metrics.cpuCores < 1 ||
      !Number.isSafeInteger(metrics.memoryTotalMb) || metrics.memoryTotalMb < 1 ||
      !Number.isSafeInteger(metrics.memoryAvailableMb) || metrics.memoryAvailableMb < 0 ||
      !Number.isSafeInteger(metrics.diskTotalMb) || metrics.diskTotalMb < 1 ||
      !Number.isSafeInteger(metrics.diskAvailableMb) || metrics.diskAvailableMb < 0 ||
      typeof metrics.dataDirectory !== "string" || typeof metrics.sampledAt !== "string" ||
      !Number.isFinite(Date.parse(metrics.sampledAt))) {
    throw new Error("Node agent returned an invalid metrics document");
  }
  return metrics;
}
