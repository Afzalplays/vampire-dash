"use client";

import { useCallback, useEffect, useState } from "react";

type Node = {
  id: string; name: string; location: string; endpoint: string; minecraftAddress: string | null; enabled: boolean;
  memoryMb: number; diskMb: number; reservedMemoryMb: number; reservedDiskMb: number;
  allocationStart: number; allocationEnd: number; healthStatus: string; lastSeenAt: string | null;
  agentVersion: string | null; cpuCores: number | null; lastHealthError: string | null;
  _count: { servers: number; reservations: number; allocations: number };
};
type UncertainJob = { id: string; status: string; attempts: number; lastErrorCode: string | null; operation: { id: string; serverId: string; type: string; status: string; errorCode: string | null; errorMessage: string | null } };
async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, { ...init, credentials: "include", headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) } });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `Request failed (${response.status})`);
  return data as T;
}
const panel: React.CSSProperties = { background: "#14141b", border: "1px solid #292933", borderRadius: 12, padding: 18 };
const field: React.CSSProperties = { width: "100%", boxSizing: "border-box", background: "#09090d", border: "1px solid #3f3f46", borderRadius: 7, color: "#fff", padding: 10, marginTop: 5 };
const action: React.CSSProperties = { background: "#29212a", border: "1px solid #69313a", borderRadius: 7, color: "#fff", padding: "8px 11px", cursor: "pointer" };
const label: React.CSSProperties = { display: "block", color: "#d4d4d8", fontSize: 12, marginBottom: 10 };
export default function AdminNodesPage() {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [token, setToken] = useState("");
  const [uncertain, setUncertain] = useState<Record<string, UncertainJob[]>>({});
  const [form, setForm] = useState({ name: "", location: "India", endpoint: "", minecraftAddress: "", memoryMb: "8192", diskMb: "81920", allocationStart: "25565", allocationEnd: "25664" });
  const load = useCallback(async () => {
    setLoading(true); setError("");
    try { const result = await api<{ nodes: Node[] }>("/admin/nodes"); setNodes(result.nodes); }
    catch (e) { setError(e instanceof Error ? e.message : "Unable to load nodes"); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  async function run(nodeId: string, actionName: "check" | "enable" | "disable" | "rotate-credentials" | "uncertain-jobs") {
    setBusy(nodeId + actionName); setError(""); setNotice(""); setToken("");
    try {
      if (actionName === "uncertain-jobs") {
        const result = await api<{ jobs: UncertainJob[]; warning: string }>(`/admin/nodes/${encodeURIComponent(nodeId)}/uncertain-jobs`);
        setUncertain((old) => ({ ...old, [nodeId]: result.jobs })); setNotice(result.warning);
      } else {
        const result = await api<{ enabled?: boolean; agentToken?: string; node?: Node }>(`/admin/nodes/${encodeURIComponent(nodeId)}/${actionName}`, { method: "POST", body: "{}" });
        if (result.agentToken) setToken(result.agentToken);
        setNotice(actionName === "check" ? "mTLS health check succeeded." : actionName === "enable" ? "Node enabled." : actionName === "disable" ? "Node disabled." : "Credentials rotated. Install the new token on the node; it is shown only once.");
      }
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Node action failed"); }
    finally { setBusy(""); }
  }
  async function createNode(event: React.FormEvent) {
    event.preventDefault(); setBusy("create"); setError(""); setNotice(""); setToken("");
    try {
      const result = await api<{ node: Node; agentToken: string; warning: string }>("/admin/nodes", { method: "POST", body: JSON.stringify({
        name: form.name, location: form.location, endpoint: form.endpoint, minecraftAddress: form.minecraftAddress || undefined,
        memoryMb: Number(form.memoryMb), diskMb: Number(form.diskMb),
        allocationStart: Number(form.allocationStart), allocationEnd: Number(form.allocationEnd),
      }) });
      setToken(result.agentToken); setNotice(result.warning); setForm((old) => ({ ...old, name: "", endpoint: "", minecraftAddress: "" })); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Node registration failed"); }
    finally { setBusy(""); }
  }
  const copyToken = async () => { try { await navigator.clipboard.writeText(token); setNotice("Agent token copied."); } catch { setError("Clipboard access failed; copy the token manually."); } };
  return <main style={{ minHeight: "100vh", background: "#09090d", color: "#f4f4f5", padding: "28px clamp(16px,4vw,56px)", fontFamily: "Arial,sans-serif" }}>
    <div style={{ maxWidth: 1200, margin: "0 auto" }}>
      <div style={{ display: "flex", gap: 14 }}><a href="/" style={{ color: "#f87171", textDecoration: "none" }}>← Control center</a><a href="/admin/releases" style={{ color: "#a1a1aa", textDecoration: "none" }}>Software releases →</a></div>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap", margin: "22px 0" }}>
        <div><small style={{ color: "#f87171", letterSpacing: 2 }}>VAMPIRE DASH / ADMIN</small><h1 style={{ margin: "7px 0", fontSize: 32 }}>Node management</h1><p style={{ color: "#a1a1aa", margin: 0 }}>Register nodes, verify mTLS, activate workers and inspect uncertain operations.</p></div>
        <button style={action} onClick={() => void load()}>Refresh nodes</button>
      </header>
      {error && <div role="alert" style={{ ...panel, borderColor: "#7f1d1d", color: "#fecaca", marginBottom: 14 }}>{error}</div>}
      {notice && <div role="status" style={{ ...panel, borderColor: "#166534", color: "#86efac", marginBottom: 14 }}>{notice}</div>}
      {token && <section style={{ ...panel, marginBottom: 16, borderColor: "#854d0e" }}><b>One-time node agent token</b><p style={{ color: "#fbbf24", fontSize: 13 }}>Copy this token now and store it in a root-only environment file on the node. It will not be shown again.</p><code style={{ display: "block", overflowWrap: "anywhere", background: "#09090d", padding: 12, borderRadius: 7 }}>{token}</code><button style={{ ...action, marginTop: 10 }} onClick={() => void copyToken()}>Copy token</button></section>}
      <section style={{ ...panel, marginBottom: 18 }}>
        <h2 style={{ marginTop: 0, fontSize: 18 }}>Register a node</h2>
        <form onSubmit={createNode} style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 10 }}>
          <label style={label}>Node name<input required minLength={2} maxLength={64} style={field} value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}/></label>
          <label style={label}>Location<input required minLength={2} maxLength={64} style={field} value={form.location} onChange={e => setForm({ ...form, location: e.target.value })}/></label>
          <label style={{ ...label, gridColumn: "span 2" }}>HTTPS agent endpoint<input required type="url" placeholder="https://node.example.com:8081" style={field} value={form.endpoint} onChange={e => setForm({ ...form, endpoint: e.target.value })}/></label><label style={label}>Public Minecraft address<input placeholder="node.example.com or public IP" style={field} value={form.minecraftAddress} onChange={e => setForm({ ...form, minecraftAddress: e.target.value })}/></label>
          <label style={label}>RAM (MiB)<input required type="number" min="1" style={field} value={form.memoryMb} onChange={e => setForm({ ...form, memoryMb: e.target.value })}/></label>
          <label style={label}>Disk (MiB)<input required type="number" min="1" style={field} value={form.diskMb} onChange={e => setForm({ ...form, diskMb: e.target.value })}/></label>
          <label style={label}>First TCP port<input required type="number" min="1024" max="65535" style={field} value={form.allocationStart} onChange={e => setForm({ ...form, allocationStart: e.target.value })}/></label>
          <label style={label}>Last TCP port<input required type="number" min="1024" max="65535" style={field} value={form.allocationEnd} onChange={e => setForm({ ...form, allocationEnd: e.target.value })}/></label>
          <div style={{ alignSelf: "end" }}><button disabled={busy === "create"} style={{ ...action, opacity: busy === "create" ? .5 : 1 }}>{busy === "create" ? "Registering…" : "Register node"}</button></div>
        </form>
      </section>
      <section style={{ display: "grid", gap: 12 }}>
        {loading ? <p style={{ color: "#a1a1aa" }}>Loading nodes…</p> : nodes.length === 0 ? <div style={panel}>No nodes registered yet.</div> : nodes.map(node => <article key={node.id} style={panel}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12, flexWrap: "wrap" }}>
            <div><h2 style={{ margin: "0 0 6px", fontSize: 19 }}>{node.name}</h2><p style={{ margin: 0, color: "#a1a1aa", fontSize: 13 }}>{node.location} · Agent: {node.endpoint} · Minecraft: {node.minecraftAddress || "not set"}</p><p style={{ margin: "8px 0", fontSize: 13 }}>Health: <b style={{ color: node.healthStatus === "ONLINE" ? "#86efac" : "#fbbf24" }}>{node.healthStatus}</b> · {node.enabled ? "Enabled" : "Disabled"} · Agent {node.agentVersion || "unknown"}</p></div>
            <div style={{ display: "flex", gap: 7, flexWrap: "wrap" }}>
              <button style={action} disabled={!!busy} onClick={() => void run(node.id, "check")}>Check mTLS</button>
              {node.enabled ? <button style={action} disabled={!!busy} onClick={() => void run(node.id, "disable")}>Disable</button> : <button style={action} disabled={!!busy || node.healthStatus !== "ONLINE"} onClick={() => void run(node.id, "enable")}>Enable</button>}
              <button style={action} disabled={!!busy} onClick={() => void run(node.id, "rotate-credentials")}>Rotate token</button>
              <button style={action} disabled={!!busy} onClick={() => void run(node.id, "uncertain-jobs")}>Uncertain jobs</button>
            </div>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(130px,1fr))", gap: 8, marginTop: 14 }}>
            {[["RAM", `${node.memoryMb} MiB`], ["Disk", `${node.diskMb} MiB`], ["CPU cores", node.cpuCores ?? "Unknown"], ["TCP range", `${node.allocationStart}-${node.allocationEnd}`], ["Servers", node._count.servers], ["Ports used", node._count.allocations]].map(([k,v]) => <div key={String(k)} style={{ background: "#09090d", borderRadius: 8, padding: 10 }}><small style={{ color: "#a1a1aa" }}>{k}</small><div style={{ marginTop: 5, fontWeight: 700 }}>{v}</div></div>)}
          </div>
          {node.lastHealthError && <p style={{ color: "#fca5a5", fontSize: 12 }}>{node.lastHealthError}</p>}
          {uncertain[node.id] && <div style={{ marginTop: 12, paddingTop: 12, borderTop: "1px solid #292933" }}><b>Uncertain jobs ({uncertain[node.id].length})</b>{uncertain[node.id].length === 0 ? <p style={{ color: "#a1a1aa" }}>No uncertain jobs.</p> : uncertain[node.id].map(job => <p key={job.id} style={{ color: "#fbbf24", fontSize: 12, overflowWrap: "anywhere" }}>{job.operation.type} · server {job.operation.serverId} · {job.lastErrorCode || job.operation.errorCode || job.status}. Do not automatically retry.</p>)}</div>}
        </article>)}
      </section>
    </div>
  </main>;
}
