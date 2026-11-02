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

function getJson<T>(url: URL, token: string, options: { ca: Buffer; cert: Buffer; key: Buffer; timeoutMs?: number }): Promise<T> {
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
        if (size > 128 * 1024) {
          req.destroy(new Error("Node agent response exceeded 128 KiB"));
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


export interface NodeAgentConsole {
  nodeId: string;
  serverId: string;
  output: string;
  sampledAt: string;
}


function postJson<T>(url: URL, token: string, body: unknown, options: { ca: Buffer; cert: Buffer; key: Buffer }): Promise<T> {
  return new Promise((resolve, reject) => {
    const encoded = Buffer.from(JSON.stringify(body), "utf8");
    const req = httpsRequest(url, {
      method: "POST",
      headers: { authorization: `Bearer ${token}`, accept: "application/json", "content-type": "application/json", "content-length": encoded.length },
      ca: options.ca, cert: options.cert, key: options.key, rejectUnauthorized: true,
      servername: url.hostname, timeout: options.timeoutMs ?? env.NODE_AGENT_TIMEOUT_MS,
    }, (res) => {
      const chunks: Buffer[] = [];
      let size = 0;
      res.on("data", (chunk: Buffer) => {
        size += chunk.length;
        if (size > 128 * 1024) { req.destroy(new Error("Node agent response exceeded 128 KiB")); return; }
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
    req.write(encoded);
    req.end();
  });
}

export async function sendNodeConsoleCommand(endpoint: string, encrypted: { ciphertext: string; nonce: string; authTag: string }, serverId: string, command: string): Promise<{ nodeId: string; serverId: string; output: string; executedAt: string }> {
  const normalized = command.trim();
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(serverId) || !normalized || Buffer.byteLength(normalized, "utf8") > 512 || /[\u0000-\u001f\u007f]/.test(normalized)) {
    throw new Error("Invalid console command");
  }
  if (!env.NODE_AGENT_CA_FILE || !env.NODE_AGENT_CLIENT_CERT_FILE || !env.NODE_AGENT_CLIENT_KEY_FILE) {
    throw new Error("Panel mTLS client is not configured");
  }
  const base = new URL(endpoint);
  if (base.protocol !== "https:" || base.username || base.password || base.search || base.hash) throw new Error("Registered node endpoint is not a valid HTTPS origin");
  const [ca, cert, key] = await Promise.all([readFile(env.NODE_AGENT_CA_FILE), readFile(env.NODE_AGENT_CLIENT_CERT_FILE), readFile(env.NODE_AGENT_CLIENT_KEY_FILE)]);
  const token = decryptNodeCredential(encrypted);
  const url = new URL(`/v1/servers/${encodeURIComponent(serverId)}/console`, base);
  const result = await postJson<{ nodeId: string; serverId: string; output: string; executedAt: string }>(url, token, { command: normalized }, { ca, cert, key, timeoutMs: 10_000 });
  if (!result || typeof result.nodeId !== "string" || result.serverId !== serverId || typeof result.output !== "string" ||
      Buffer.byteLength(result.output, "utf8") > 8 * 1024 || typeof result.executedAt !== "string" || !Number.isFinite(Date.parse(result.executedAt))) {
    throw new Error("Node agent returned an invalid console command result");
  }
  return result;
}

export async function fetchNodeConsole(endpoint: string, encrypted: { ciphertext: string; nonce: string; authTag: string }, serverId: string, tail = 100): Promise<NodeAgentConsole> {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(serverId) || !Number.isSafeInteger(tail) || tail < 1 || tail > 200) {
    throw new Error("Invalid server console request");
  }
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
  const url = new URL(`/v1/servers/${encodeURIComponent(serverId)}/console?tail=${tail}`, base);
  const result = await getJson<NodeAgentConsole>(url, token, { ca, cert, key });
  if (!result || typeof result.nodeId !== "string" || result.serverId !== serverId ||
      typeof result.output !== "string" || Buffer.byteLength(result.output, "utf8") > 48 * 1024 ||
      typeof result.sampledAt !== "string" || !Number.isFinite(Date.parse(result.sampledAt))) {
    throw new Error("Node agent returned an invalid console document");
  }
  return result;
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
