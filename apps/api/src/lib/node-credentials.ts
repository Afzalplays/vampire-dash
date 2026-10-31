import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";
import { env } from "../env.js";

export interface EncryptedCredential {
  ciphertext: string;
  nonce: string;
  authTag: string;
}

const credentialKey = (): Buffer => {
  if (!env.NODE_CREDENTIALS_KEY) {
    throw new Error("NODE_CREDENTIALS_KEY is required before registering compute nodes");
  }
  const key = Buffer.from(env.NODE_CREDENTIALS_KEY, "base64");
  if (key.length !== 32) throw new Error("NODE_CREDENTIALS_KEY must decode to exactly 32 bytes");
  return key;
};

/** Encrypt a node bearer token with AES-256-GCM and a fresh 96-bit nonce. */
export function encryptNodeCredential(token: string): EncryptedCredential {
  const nonce = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", credentialKey(), nonce);
  const ciphertext = Buffer.concat([cipher.update(token, "utf8"), cipher.final()]);
  return {
    ciphertext: ciphertext.toString("base64"),
    nonce: nonce.toString("base64"),
    authTag: cipher.getAuthTag().toString("base64"),
  };
}

/** Decrypt only inside the API process; never return this value from admin list APIs. */
export function decryptNodeCredential(value: EncryptedCredential): string {
  const decipher = createDecipheriv("aes-256-gcm", credentialKey(), Buffer.from(value.nonce, "base64"));
  decipher.setAuthTag(Buffer.from(value.authTag, "base64"));
  return Buffer.concat([
    decipher.update(Buffer.from(value.ciphertext, "base64")),
    decipher.final(),
  ]).toString("utf8");
}
