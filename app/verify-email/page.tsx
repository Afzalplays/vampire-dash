"use client";
import { FormEvent, useState } from "react";
import Link from "next/link";
const API = process.env.NEXT_PUBLIC_API_URL ?? "http://127.0.0.1:4000/api";
export default function VerifyEmailPage() {
  const [email, setEmail] = useState("");
  const [otp, setOtp] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function verify(e: FormEvent) {
    e.preventDefault(); setBusy(true); setError(""); setMessage("");
    try {
      const response = await fetch(API + "/auth/verify-email", { method: "POST", credentials: "include", headers: { "content-type": "application/json" }, body: JSON.stringify({ email, otp }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error ?? "Verification failed");
      location.href = "/";
    } catch (err) { setError(err instanceof Error ? err.message : "Verification failed"); }
    finally { setBusy(false); }
  }
  async function resend() {
    setError(""); setMessage("");
    try {
      const response = await fetch(API + "/auth/resend-verification", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email }) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error ?? "Could not resend code");
      setMessage(data.message);
    } catch (err) { setError(err instanceof Error ? err.message : "Could not resend code"); }
  }
  return <main className="auth-wrap"><form className="auth-card" onSubmit={verify}>
    <Link className="auth-brand" href="/">VAMPIREHOSTING</Link><small>EMAIL SECURITY</small><h1>Verify your email</h1>
    <p>Enter the 6-digit code sent to your email address. The code expires in 10 minutes.</p>
    <label>Email address<input type="email" autoComplete="email" required value={email} onChange={e => setEmail(e.target.value)} /></label>
    <label>Verification code<input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" minLength={6} maxLength={6} required value={otp} onChange={e => setOtp(e.target.value.replace(/\D/g, "").slice(0, 6))} /></label>
    {error && <div className="auth-error">{error}</div>}{message && <p role="status">{message}</p>}
    <button disabled={busy}>{busy ? "Verifying…" : "Verify email"}</button>
    <button type="button" className="auth-secondary" onClick={resend}>Resend code</button>
    <div className="auth-foot"><Link href="/login">Back to sign in</Link></div>
  </form></main>;
}
