import nodemailer from "nodemailer";
import { env } from "../env.js";

const transport = nodemailer.createTransport({
  host: env.SMTP_HOST,
  port: env.SMTP_PORT,
  secure: env.SMTP_SECURE,
  auth: env.SMTP_USER ? { user: env.SMTP_USER, pass: env.SMTP_PASSWORD } : undefined,
});

export async function sendMail(input: { to: string; subject: string; text: string; html?: string }) {
  if (!env.SMTP_HOST || !env.SMTP_FROM) throw new Error("SMTP is not configured; email delivery is unavailable");
  await transport.sendMail({ from: env.SMTP_FROM, to: input.to, subject: input.subject, text: input.text, html: input.html });
}
