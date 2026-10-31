ALTER TABLE "MinecraftNode"
  ADD COLUMN "healthStatus" TEXT NOT NULL DEFAULT 'UNKNOWN',
  ADD COLUMN "lastSeenAt" TIMESTAMP(3),
  ADD COLUMN "agentVersion" TEXT,
  ADD COLUMN "cpuCores" INTEGER,
  ADD COLUMN "memoryAvailableMb" BIGINT,
  ADD COLUMN "diskAvailableMb" BIGINT,
  ADD COLUMN "metricsSampledAt" TIMESTAMP(3),
  ADD COLUMN "lastHealthError" TEXT;
