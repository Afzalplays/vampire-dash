import "dotenv/config";
import { z } from "zod";

const schema = z.object({
  NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  API_HOST: z.string().default("127.0.0.1"),
  API_PORT: z.coerce.number().int().min(1).max(65535).default(4000),
  WEB_ORIGIN: z.string().url().default("http://localhost:3000"),
  DATABASE_URL: z.string().min(1),
  SESSION_SECRET: z.string().min(32),
  NODE_CREDENTIALS_KEY: z.string().default(""),
  NODE_AGENT_CA_FILE: z.string().default(""),
  NODE_AGENT_CLIENT_CERT_FILE: z.string().default(""),
  NODE_AGENT_CLIENT_KEY_FILE: z.string().default(""),
  NODE_AGENT_TIMEOUT_MS: z.coerce.number().int().min(1000).max(30000).default(5000),
  NODE_HEALTH_INTERVAL_MS: z.coerce.number().int().min(15000).max(600000).default(60000),
  COOKIE_SECURE: z.enum(["true", "false"]).default("false").transform(v => v === "true"),
  SMTP_HOST: z.string().default(""),
  SMTP_PORT: z.coerce.number().int().min(1).max(65535).default(587),
  SMTP_SECURE: z.enum(["true", "false"]).default("false").transform(v => v === "true"),
  SMTP_USER: z.string().default(""),
  SMTP_PASSWORD: z.string().default(""),
  SMTP_FROM: z.string().default(""),
});

export const env = schema.superRefine((value, ctx) => {
  if (value.NODE_ENV === "production" && !value.WEB_ORIGIN.startsWith("https://")) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["WEB_ORIGIN"], message: "Production WEB_ORIGIN must use HTTPS" });
  }
  if (value.NODE_ENV === "production" && !value.COOKIE_SECURE) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["COOKIE_SECURE"], message: "COOKIE_SECURE=true is mandatory in production" });
  }
  const mtlsValues = [value.NODE_AGENT_CA_FILE, value.NODE_AGENT_CLIENT_CERT_FILE, value.NODE_AGENT_CLIENT_KEY_FILE];
  if (mtlsValues.some(Boolean) && !mtlsValues.every(Boolean)) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["NODE_AGENT_CA_FILE"], message: "Configure the node-agent CA, client certificate and client key together" });
  }
  if (value.NODE_ENV === "production" && !mtlsValues.every(Boolean)) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["NODE_AGENT_CA_FILE"], message: "Production node checks require panel mTLS CA, client certificate and key files" });
  }
  if (value.NODE_CREDENTIALS_KEY) {
    try {
      if (Buffer.from(value.NODE_CREDENTIALS_KEY, "base64").length !== 32) throw new Error();
    } catch {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["NODE_CREDENTIALS_KEY"], message: "NODE_CREDENTIALS_KEY must be base64-encoded 32 random bytes" });
    }
  }
}).parse(process.env);
