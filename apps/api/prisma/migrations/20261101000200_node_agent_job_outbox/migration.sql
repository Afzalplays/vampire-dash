CREATE TYPE "AgentJobStatus" AS ENUM ('PENDING', 'LEASED', 'ACKNOWLEDGED', 'FAILED', 'DEAD');

CREATE TABLE "NodeAgentJob" (
  "id" TEXT NOT NULL,
  "operationId" TEXT NOT NULL,
  "nodeId" TEXT NOT NULL,
  "status" "AgentJobStatus" NOT NULL DEFAULT 'PENDING',
  "nonce" TEXT NOT NULL,
  "attempts" INTEGER NOT NULL DEFAULT 0,
  "maxAttempts" INTEGER NOT NULL DEFAULT 5,
  "availableAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "leaseTokenHash" TEXT,
  "leaseExpiresAt" TIMESTAMP(3),
  "lastErrorCode" TEXT,
  "lastError" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "NodeAgentJob_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "NodeAgentJob_attempts_check" CHECK ("attempts" >= 0 AND "maxAttempts" BETWEEN 1 AND 20)
);

CREATE UNIQUE INDEX "NodeAgentJob_operationId_key" ON "NodeAgentJob"("operationId");
CREATE UNIQUE INDEX "NodeAgentJob_nonce_key" ON "NodeAgentJob"("nonce");
CREATE INDEX "NodeAgentJob_nodeId_status_availableAt_idx" ON "NodeAgentJob"("nodeId", "status", "availableAt");
CREATE INDEX "NodeAgentJob_status_leaseExpiresAt_idx" ON "NodeAgentJob"("status", "leaseExpiresAt");

ALTER TABLE "NodeAgentJob" ADD CONSTRAINT "NodeAgentJob_operationId_fkey"
FOREIGN KEY ("operationId") REFERENCES "MinecraftServerOperation"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "NodeAgentJob" ADD CONSTRAINT "NodeAgentJob_nodeId_fkey"
FOREIGN KEY ("nodeId") REFERENCES "MinecraftNode"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
