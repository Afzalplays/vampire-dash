"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";

type Server = {id:string;name:string;software:string;version:string;memoryMb:number;diskMb:number;cpuPercent:number;status:string;expiresAt:string|null;allocation:{port:number;protocol:string}|null;node:{name:string;location:string;minecraftAddress:string|null;healthStatus:string;lastSeenAt:string|null}|null;access:{role:string;permissions:Record<string,boolean>}};
type Operation = {id:string;type:string;status:string;payload:unknown;errorMessage:string|null;createdAt:string;startedAt:string|null;finishedAt:string|null};
async function api<T>(path:string,init?:RequestInit):Promise<T>{const r=await fetch(`/api${path}`,{...init,credentials:"include",headers:{"Content-Type":"application/json",...(init?.headers??{})}});const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d.error||`Request failed (${r.status})`);return d as T;}
const box:React.CSSProperties={background:"#14141b",border:"1px solid #292933",borderRadius:12,padding:20};
const input:React.CSSProperties={display:"block",width:"100%",background:"#09090d",border:"1px solid #3f3f46",borderRadius:7,color:"#fff",padding:10,marginTop:6};
const button:React.CSSProperties={background:"#29212a",border:"1px solid #69313a",borderRadius:7,color:"#fff",padding:"10px 14px",cursor:"pointer"};
const label:React.CSSProperties={display:"block",fontSize:12,color:"#d4d4d8",marginBottom:12};
const pretty=(s:string)=>s.replaceAll("_"," ").toLowerCase().replace(/\b\w/g,c=>c.toUpperCase());
export default function ServerDetailPage(){
 const params=useParams<{serverId:string}>();const id=params.serverId;
 const [server,setServer]=useState<Server|null>(null);const [ops,setOps]=useState<Operation[]>([]);
 const [loading,setLoading]=useState(true);const [error,setError]=useState("");const [notice,setNotice]=useState("");const [busy,setBusy]=useState(false);
 const [motd,setMotd]=useState("");const [difficulty,setDifficulty]=useState("normal");const [maxPlayers,setMaxPlayers]=useState("20");const [viewDistance,setViewDistance]=useState("10");\n const [consoleOutput,setConsoleOutput]=useState("");const [consoleError,setConsoleError]=useState("");const [consoleBusy,setConsoleBusy]=useState(false);const [consoleAuto,setConsoleAuto]=useState(true);
 const load=useCallback(async()=>{setLoading(true);setError("");try{const [s,o]=await Promise.all([api<{server:Server}>(`/servers/${encodeURIComponent(id)}`),api<{operations:Operation[]}>(`/servers/${encodeURIComponent(id)}/operations`)]);setServer(s.server);setOps(o.operations);}catch(e){setError(e instanceof Error?e.message:"Unable to load server");}finally{setLoading(false);}},[id]);
 useEffect(()=>{if(id)void load();},[id,load]); 
 const loadConsole=useCallback(async()=>{
  if(!id)return;
  setConsoleBusy(true);setConsoleError("");
  try{const result=await api<{output:string;sampledAt:string}>(`/servers/${encodeURIComponent(id)}/console?tail=150`);setConsoleOutput(result.output);}
  catch(e){setConsoleError(e instanceof Error?e.message:"Console is unavailable");}
  finally{setConsoleBusy(false);}
 },[id]);
 useEffect(()=>{if(id&&server?.status==="RUNNING"&&server.access.permissions.viewConsole)void loadConsole();},[id,server?.status,server?.access.permissions.viewConsole,loadConsole]);
 useEffect(()=>{
  if(!consoleAuto||server?.status!=="RUNNING"||!server.access.permissions.viewConsole)return;
  const timer=window.setInterval(()=>{void loadConsole();},5000);
  return()=>window.clearInterval(timer);
 },[consoleAuto,server?.status,server?.access.permissions.viewConsole,loadConsole]);

 async function queue(type:string,payload:Record<string,unknown>={}){setBusy(true);setError("");setNotice("");try{const result=await api<{execution:string}>(`/servers/${encodeURIComponent(id)}/operations`,{method:"POST",body:JSON.stringify({type,payload,idempotencyKey:crypto.randomUUID()})});setNotice(result.execution==="QUEUED_FOR_NODE_AGENT"?"Request saved in the node-agent queue. It runs only when the assigned node is enabled and its worker is configured.":"Request accepted.");await load();}catch(e){setError(e instanceof Error?e.message:"Unable to queue operation");}finally{setBusy(false);}}
 if(loading&&!server)return <main style={{background:"#09090d",color:"#fff",minHeight:"100vh",padding:32}}>Loading server…</main>;
 if(!server)return <main style={{background:"#09090d",color:"#fff",minHeight:"100vh",padding:32}}><a href="/servers" style={{color:"#f87171"}}>← My servers</a><h1>Server unavailable</h1><p>{error||"Server not found."}</p></main>;
 const permissions=server.access.permissions;
 return <main style={{minHeight:"100vh",background:"#09090d",color:"#f4f4f5",padding:"32px clamp(18px,5vw,64px)",fontFamily:"Arial,sans-serif"}}><div style={{maxWidth:1100,margin:"0 auto"}}>
 <a href="/servers" style={{color:"#f87171",textDecoration:"none"}}>← My servers</a>
 <header style={{display:"flex",justifyContent:"space-between",alignItems:"center",gap:14,flexWrap:"wrap",margin:"22px 0"}}><div><small style={{color:"#f87171",letterSpacing:2}}>SERVER CONTROL</small><h1 style={{fontSize:34,margin:"8px 0"}}>{server.name}</h1><p style={{color:"#a1a1aa",margin:0}}>{pretty(server.software)} · Minecraft {server.version} · {pretty(server.status)}</p></div><button style={button} onClick={()=>void load()}>Refresh</button></header>
 {error&&<div role="alert" style={{...box,borderColor:"#7f1d1d",color:"#fecaca",marginBottom:14}}>{error}</div>}{notice&&<div role="status" style={{...box,borderColor:"#166534",color:"#86efac",marginBottom:14}}>{notice}</div>}
 <section style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(180px,1fr))",gap:12,marginBottom:18}}>
 {[["Memory",`${(server.memoryMb/1024).toFixed(1)} GB`],["Disk",`${(server.diskMb/1024).toFixed(1)} GB`],["CPU",`${server.cpuPercent}%`],["Node",server.node?.name||"Unassigned"],["Address",server.allocation?(server.node?.minecraftAddress?`${server.node.minecraftAddress}:${server.allocation.port}`:"Public address not configured"):"Not allocated"]].map(([k,v])=><article key={k} style={box}><small style={{color:"#a1a1aa"}}>{k}</small><h2 style={{fontSize:23,margin:"10px 0 0"}}>{v}</h2></article>)}
 </section>
 <section style={{...box,marginBottom:16}}><h2 style={{marginTop:0,fontSize:18}}>Power controls</h2><p style={{color:"#a1a1aa",fontSize:13}}>These actions enter the signed node-agent queue. They execute only if the assigned node is enabled, reachable and configured with AGENT_ENABLE_JOB_EXECUTION=true.</p><div style={{display:"flex",gap:10,flexWrap:"wrap"}}>{["START","RESTART","STOP"].map(action=><button key={action} disabled={busy||!permissions.power} onClick={()=>void queue(action)} style={{...button,opacity:(busy || !permissions.power) ? 0.6 : 1}}>{pretty(action)}</button>)}</div>{!permissions.power&&<small style={{color:"#fbbf24"}}>Your account does not have power permission.</small>}</section>
 <section style={{...box,marginBottom:16}}><div style={{display:"flex",justifyContent:"space-between",alignItems:"center",gap:12,flexWrap:"wrap"}}><div><h2 style={{margin:"0 0 5px",fontSize:18}}>Live console</h2><small style={{color:"#a1a1aa"}}>Latest 150 lines · refreshes every 5 seconds</small></div><div style={{display:"flex",gap:8,alignItems:"center"}}><label style={{display:"flex",gap:6,alignItems:"center",fontSize:12,color:"#d4d4d8"}}><input type="checkbox" checked={consoleAuto} onChange={e=>setConsoleAuto(e.target.checked)}/> Auto refresh</label><button style={button} disabled={consoleBusy||!permissions.viewConsole||server.status!=="RUNNING"} onClick={()=>void loadConsole()}>{consoleBusy?"Loading…":"Refresh logs"}</button></div></div>{!permissions.viewConsole?<p style={{color:"#fbbf24",fontSize:13}}>Your account does not have console permission.</p>:server.status!=="RUNNING"?<p style={{color:"#a1a1aa",fontSize:13}}>Console output is available while the server is running.</p>:null}{consoleError&&<p role="alert" style={{color:"#fca5a5",fontSize:12}}>{consoleError}</p>}<pre aria-label="Minecraft server console output" aria-live="polite" style={{margin:"14px 0 0",padding:14,minHeight:260,maxHeight:460,overflow:"auto",background:"#050507",border:"1px solid #30303a",borderRadius:8,color:"#d4d4d8",fontSize:12,lineHeight:1.55,whiteSpace:"pre-wrap",overflowWrap:"anywhere",fontFamily:"ui-monospace,Consolas,monospace"}}>{consoleOutput||"Waiting for server output…"}</pre></section>
 <section style={{...box,marginBottom:16}}><h2 style={{marginTop:0,fontSize:18}}>Server properties</h2><p style={{color:"#a1a1aa",fontSize:13}}>Property changes are temporarily unavailable. They will be enabled after the node file adapter supports atomic, verified updates.</p></section>
 <section style={box}><div style={{display:"flex",justifyContent:"space-between",alignItems:"center",gap:12}}><h2 style={{fontSize:18,margin:0}}>Recent operations</h2><span style={{fontSize:11,color:"#a1a1aa"}}>Latest 50</span></div>{ops.length===0?<p style={{color:"#a1a1aa"}}>No operations recorded.</p>:<div style={{marginTop:14}}>{ops.map(op=><div key={op.id} style={{padding:"13px 0",borderTop:"1px solid #292933",display:"flex",justifyContent:"space-between",gap:12,flexWrap:"wrap"}}><div><b>{pretty(op.type)}</b><div style={{fontSize:11,color:"#a1a1aa",marginTop:5}}>{new Date(op.createdAt).toLocaleString()}</div>{op.errorMessage&&<small style={{color:"#fca5a5"}}>{op.errorMessage}</small>}</div><span style={{fontSize:12,color:op.status==="SUCCEEDED"?"#86efac":"#fbbf24"}}>{pretty(op.status)}</span></div>)}</div>}</section>
 </div></main>;
}
