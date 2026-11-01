"use client";

import { useCallback, useEffect, useState } from "react";

type Release = { id: string; software: string; minecraftVersion: string; softwareVersion: string; javaMajor: number; dockerImage: string; enabled: boolean; recommended: boolean; createdAt: string };
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
const software = ["VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD"];
export default function AdminReleasesPage() {
  const [releases, setReleases] = useState<Release[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [form, setForm] = useState({ software: "PAPER", minecraftVersion: "1.21.1", softwareVersion: "1.21.1", javaMajor: "21", dockerImage: "", recommended: true });
  const load = useCallback(async () => {
    setLoading(true); setError("");
    try { const result = await api<{ releases: Release[] }>("/admin/software/releases"); setReleases(result.releases); }
    catch (e) { setError(e instanceof Error ? e.message : "Unable to load releases"); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  async function createRelease(event: React.FormEvent) {
    event.preventDefault(); setBusy("create"); setError(""); setNotice("");
    try {
      await api("/admin/software/releases", { method: "POST", body: JSON.stringify({ ...form, javaMajor: Number(form.javaMajor), enabled: true }) });
      setNotice("Pinned software release added and enabled."); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Release creation failed"); }
    finally { setBusy(""); }
  }
  async function toggle(release: Release) {
    setBusy(release.id); setError(""); setNotice("");
    try {
      await api(`/admin/software/releases/${encodeURIComponent(release.id)}`, { method: "PATCH", body: JSON.stringify({ enabled: !release.enabled }) });
      setNotice(`Release ${release.enabled ? "disabled" : "enabled"}.`); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Release update failed"); }
    finally { setBusy(""); }
  }
  return <main style={{ minHeight: "100vh", background: "#09090d", color: "#f4f4f5", padding: "28px clamp(16px,4vw,56px)", fontFamily: "Arial,sans-serif" }}>
    <div style={{ maxWidth: 1200, margin: "0 auto" }}>
      <div style={{ display: "flex", gap: 14, flexWrap: "wrap" }}><a href="/admin/nodes" style={{ color: "#f87171", textDecoration: "none" }}>← Admin nodes</a><a href="/" style={{ color: "#a1a1aa", textDecoration: "none" }}>Control center</a></div>
      <header style={{ margin: "22px 0" }}><small style={{ color: "#f87171", letterSpacing: 2 }}>VAMPIRE DASH / ADMIN</small><h1 style={{ margin: "7px 0", fontSize: 32 }}>Software releases</h1><p style={{ color: "#a1a1aa", margin: 0 }}>Manage the pinned Java Minecraft images customers can provision.</p></header>
      {error && <div role="alert" style={{ ...panel, borderColor: "#7f1d1d", color: "#fecaca", marginBottom: 14 }}>{error}</div>}
      {notice && <div role="status" style={{ ...panel, borderColor: "#166534", color: "#86efac", marginBottom: 14 }}>{notice}</div>}
      <section style={{ ...panel, marginBottom: 18 }}>
        <h2 style={{ marginTop: 0, fontSize: 18 }}>Add a release</h2>
        <p style={{ color: "#fbbf24", fontSize: 13 }}>Only digest-pinned itzg/minecraft-server images are accepted for provisioning. Verify the digest from a trusted source before enabling a release.</p>
        <form onSubmit={createRelease} style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 10 }}>
          <label style={label}>Software<select style={field} value={form.software} onChange={e => setForm({ ...form, software: e.target.value })}>{software.map(value => <option key={value}>{value}</option>)}</select></label>
          <label style={label}>Minecraft version<input required maxLength={32} style={field} value={form.minecraftVersion} onChange={e => setForm({ ...form, minecraftVersion: e.target.value })}/></label>
          <label style={label}>Software version<input required maxLength={64} style={field} value={form.softwareVersion} onChange={e => setForm({ ...form, softwareVersion: e.target.value })}/></label>
          <label style={label}>Java major<input required type="number" min="8" max="30" style={field} value={form.javaMajor} onChange={e => setForm({ ...form, javaMajor: e.target.value })}/></label>
          <label style={{ ...label, gridColumn: "1 / -1" }}>Pinned Docker image<input required pattern="(docker\.io/)?itzg/minecraft-server@sha256:[a-f0-9]{64}" placeholder="itzg/minecraft-server@sha256:..." style={field} value={form.dockerImage} onChange={e => setForm({ ...form, dockerImage: e.target.value })}/></label>
          <label style={{ ...label, display: "flex", alignItems: "center", gap: 8 }}><input type="checkbox" checked={form.recommended} onChange={e => setForm({ ...form, recommended: e.target.checked })}/> Recommended release</label>
          <div style={{ alignSelf: "end" }}><button disabled={busy === "create"} style={{ ...action, opacity: busy === "create" ? .5 : 1 }}>{busy === "create" ? "Saving…" : "Add and enable"}</button></div>
        </form>
      </section>
      <section style={{ display: "grid", gap: 10 }}>
        {loading ? <p style={{ color: "#a1a1aa" }}>Loading releases…</p> : releases.length === 0 ? <div style={panel}>No releases configured.</div> : releases.map(release => <article key={release.id} style={panel}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
            <div><b>{release.software} · Minecraft {release.minecraftVersion}</b><p style={{ color: "#a1a1aa", fontSize: 12, margin: "6px 0" }}>Software {release.softwareVersion} · Java {release.javaMajor} · {release.recommended ? "Recommended" : "Standard"}</p><code style={{ fontSize: 11, overflowWrap: "anywhere" }}>{release.dockerImage}</code></div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}><span style={{ color: release.enabled ? "#86efac" : "#fbbf24", fontSize: 12 }}>{release.enabled ? "Enabled" : "Disabled"}</span><button style={action} disabled={!!busy} onClick={() => void toggle(release)}>{release.enabled ? "Disable" : "Enable"}</button></div>
          </div>
        </article>)}
      </section>
    </div>
  </main>;
}
