ALTER TABLE "MinecraftNode"
  ADD COLUMN "tokenCiphertext" TEXT,
  ADD COLUMN "tokenNonce" TEXT,
  ADD COLUMN "tokenAuthTag" TEXT;
