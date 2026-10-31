CREATE TYPE "ServerOperationType" AS ENUM ('INSTALL_PLUGIN', 'INSTALL_MOD', 'INSTALL_MODPACK', 'CHANGE_VERSION', 'RESTART', 'STOP', 'START', 'REINSTALL', 'CREATE_BACKUP', 'RESTORE_BACKUP', 'UPDATE_PROPERTIES');
CREATE TYPE "ServerOperationStatus" AS ENUM ('QUEUED', 'RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELLED');

CREATE TABLE "MinecraftServerOperation" (
  "id" TEXT NOT NULL,
  "serverId" TEXT NOT NULL,
  "actorUserId" TEXT NOT NULL,
  "type" "ServerOperationType" NOT NULL,
  "status" "ServerOperationStatus" NOT NULL DEFAULT 'QUEUED',
  "idempotencyKey" TEXT NOT NULL,
  "payload" JSONB NOT NULL,
  "result" JSONB,
  "errorCode" TEXT,
  "errorMessage" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "startedAt" TIMESTAMP(3),
  "finishedAt" TIMESTAMP(3),
  CONSTRAINT "MinecraftServerOperation_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "MinecraftServerOperation_idempotencyKey_key" ON "MinecraftServerOperation"("idempotencyKey");
CREATE INDEX "MinecraftServerOperation_serverId_createdAt_idx" ON "MinecraftServerOperation"("serverId", "createdAt");
CREATE INDEX "MinecraftServerOperation_status_createdAt_idx" ON "MinecraftServerOperation"("status", "createdAt");
ALTER TABLE "MinecraftServerOperation" ADD CONSTRAINT "MinecraftServerOperation_serverId_fkey"
FOREIGN KEY ("serverId") REFERENCES "MinecraftServer"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "MinecraftServerOperation" ADD CONSTRAINT "MinecraftServerOperation_actorUserId_fkey"
FOREIGN KEY ("actorUserId") REFERENCES "User"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
