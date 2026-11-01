"use client";

import { useCallback, useEffect, useState } from "react";

type Server = {
  id: string; name: string; software: string; version: string; memoryMb: number;
  diskMb: number; cpuPercent: number; status: string; expiresAt: string | null;
  node: { name: string; location: string; healthStatus: string; lastSeenAt: string | null } | null;
  access: { role: string; permissions: Record<string, boolean> };
};
type Entitlement = {
  id: string; expiresAt: string;
  plan: { name: string; memoryMb: number; diskMb: number; cpuPercent: number; serverSlots: number };
};
const software = ["PAPER", "PURPUR", "VANILLA", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD", "BEDROCK"];

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init, credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `Request failed (${response.status})`);
  return data as T;
}
const pretty = (value: string) => value.replaceAll("_", " ").toLowerCase().replace(/\b\w/g, c => c.toUpperCase());
const mb = (value: number) => value >= 1024 ? `${(value / 1024).toFixed(value % 1024 ? 1 : 0)} GB` : `${value} MB`;

export default function ServersPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [entitlements, setEntitlements] = useState<Entitlement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [name, setName] = useState("");
  const [version, setVersion] = useState("1.21.10");
  const [kind, setKind] = useState("PAPER");
  const [entitlementId, setEntitlementId] = useState("");
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true); setError("");
    try {
      const [serverData, planData] = await Promise.all([
        api<{servers: Server[]}>("/servers"),
        api<{entitlements: Entitlement[]}>("/plans/me/entitlement"),
      ]);
      setServers(serverData.servers);
      setEntitlements(planData.entitlements);
      setEntitlementId(current => current || planData.entitlements[0]?.id || "");
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to load servers"); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);

  async function createServer(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setCreating(true); setError(""); setNotice("");
    try {
      const result = await api<{provisioning: string}>("/servers", {
        method: "POST", body: JSON.stringify({ entitlementId, name, software: kind, version }),
      });
      setNotice(result.provisioning === "PENDING_AGENT_DISPATCH"
        ? "Server request accepted. It is pending secure node-agent provisioning; it is not running yet."
        : "Server request accepted.");
      setName(""); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Server creation failed"); }
    finally { setCreating(false); }
  }

  return <main style={{minHeight:"100vh",background:"#09090d",color:"#f4f4f5",padding:"36px clamp(18px,5vw,64px)",fontFamily:"Arial,sans-serif"}}>
    <div style={{maxWidth:1100,margin:"0 auto"}}>
      <a href="/" style={{color:"#f87171",textDecoration:"none"}}>← Vampire Hosting</a>
      <header style={{display:"flex",justifyContent:"space-between",alignItems:"end",gap:16,flexWrap:"wrap",margin:"24px 0 30px"}}>
        <div><div style={{color:"#f87171",fontSize:12,letterSpacing:3}}>CONTROL CENTER</div><h1 style={{fontSize:36,margin:"8px 0"}}>My servers</h1><p style={{color:"#a1a1aa",margin:0}}>Manage your Minecraft servers and plan resources.</p></div>
        <button onClick={() => void load()} style={buttonStyle}>Refresh</button>
      </header>
      {error && <div role="alert" style={alertStyle}>{error}</div>}
      {notice && <div role="status" style={{...alertStyle,borderColor:"#166534",color:"#86efac"}}>{notice}</div>}
      <section style={panelStyle}>
        <h2 style={{marginTop:0,fontSize:19}}>Create a server</h2>
        {entitlements.length === 0 ? <p style={{color:"#a1a1aa"}}>No active plan entitlement found. Choose or purchase a plan before creating a server.</p> :
        <form onSubmit={createServer} style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(180px,1fr))",gap:14,alignItems:"end"}}>
          <label style={labelStyle}>Server name<input required minLength={3} maxLength={32} value={name} onChange={e=>setName(e.target.value)} placeholder="My Survival Server" style={inputStyle}/></label>
          <label style={labelStyle}>Plan entitlement<select value={entitlementId} onChange={e=>setEntitlementId(e.target.value)} style={inputStyle}>{entitlements.map(e=><option key={e.id} value={e.id}>{e.plan.name} · {mb(e.plan.memoryMb)} RAM · {e.plan.serverSlots} slots</option>)}</select></label>
          <label style={labelStyle}>Software<select value={kind} onChange={e=>setKind(e.target.value)} style={inputStyle}>{software.map(s=><option key={s} value={s}>{pretty(s)}</option>)}</select></label>
          <label style={labelStyle}>Minecraft version<input required maxLength={32} value={version} onChange={e=>setVersion(e.target.value)} style={inputStyle}/></label>
          <button disabled={creating || !entitlementId} type="submit" style={{...buttonStyle,background:"#991b1b",opacity:creating?0.6:1}}>{creating?"Submitting…":"Create server"}</button>
        </form>}
      </section>
      <section style={{marginTop:28}}>
        <div style={{display:"flex",justifyContent:"space-between",alignItems:"center"}}><h2 style={{fontSize:20}}>Your servers</h2><span style={{color:"#a1a1aa",fontSize:13}}>{servers.length} total</span></div>
        {loading ? <p style={{color:"#a1a1aa"}}>Loading servers…</p> : servers.length === 0 ? <div style={{...panelStyle,textAlign:"center",padding:42}}><div style={{fontSize:32}}>◇</div><b>No servers yet</b><p style={{color:"#a1a1aa"}}>Create your first server when an active entitlement and a compatible release are available.</p></div> :
        <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(260px,1fr))",gap:14}}>{servers.map(server=><article key={server.id} style={panelStyle}><a href={`/servers/${encodeURIComponent(server.id)}`} style={{color:"#f87171",fontSize:12,textDecoration:"none"}}>Manage server →</a>
          <div style={{display:"flex",justifyContent:"space-between",gap:10}}><h3 style={{margin:"0 0 14px"}}>{server.name}</h3><span style={{fontSize:11,color:server.status==="RUNNING"?"#86efac":"#fbbf24"}}>{pretty(server.status)}</span></div>
          <p style={{color:"#a1a1aa",fontSize:13}}>{pretty(server.software)} · {server.version}</p>
          <div style={{display:"flex",gap:18,fontSize:13}}><span>{mb(server.memoryMb)} RAM</span><span>{mb(server.diskMb)} Disk</span><span>{server.cpuPercent}% CPU</span></div>
          <p style={{color:"#a1a1aa",fontSize:12,marginBottom:0}}>{server.node ? `${server.node.name} · ${server.node.location} · ${pretty(server.node.healthStatus)}` : "Node not assigned"}</p>
          <p style={{color:"#71717a",fontSize:11,marginBottom:0}}>{server.access.role==="OWNER"?"Owner access":"Shared access"}</p>
        </article>)}</div>}
      </section>
    </div>
  </main>;
}
const panelStyle: React.CSSProperties = {background:"#14141b",border:"1px solid #292933",borderRadius:14,padding:22};
const buttonStyle: React.CSSProperties = {background:"#27272f",border:"1px solid #3f3f46",borderRadius:8,color:"#fff",padding:"11px 16px",cursor:"pointer"};
const inputStyle: React.CSSProperties = {display:"block",width:"100%",marginTop:7,background:"#09090d",border:"1px solid #3f3f46",borderRadius:8,color:"#fff",padding:11};
const labelStyle: React.CSSProperties = {fontSize:12,color:"#d4d4d8"};
const alertStyle: React.CSSProperties = {background:"#271313",border:"1px solid #7f1d1d",borderRadius:8,padding:12,marginBottom:14,color:"#fecaca"};
