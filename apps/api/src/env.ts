import "dotenv/config";
import { z } from "zod";

const schema = z.object({
  NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  API_HOST: z.string().default("127.0.0.1"),
  API_PORT: z.coerce.number().int().min(1).max(65535).default(4000),
  WEB_ORIGIN: z.string().url().default("http://localhost:3000"),
  DATABASE_URL: z.string().min(1),
  SESSION_SECRET: z.string().min(32),
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
}).parse(process.env);
