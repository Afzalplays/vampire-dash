CREATE TYPE "CapacityReservationStatus" AS ENUM ('ACTIVE', 'RELEASED', 'CONSUMED', 'EXPIRED');

ALTER TABLE "MinecraftNode"
  ADD COLUMN "reservedMemoryMb" INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN "reservedDiskMb" INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN "memoryOverheadMb" INTEGER NOT NULL DEFAULT 1024,
  ADD COLUMN "diskOverheadMb" INTEGER NOT NULL DEFAULT 1024;

CREATE TABLE "NodeCapacityReservation" (
  "id" TEXT NOT NULL,
  "nodeId" TEXT NOT NULL,
  "memoryMb" INTEGER NOT NULL,
  "diskMb" INTEGER NOT NULL,
  "status" "CapacityReservationStatus" NOT NULL DEFAULT 'ACTIVE',
  "idempotencyKey" TEXT NOT NULL,
  "expiresAt" TIMESTAMP(3) NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "NodeCapacityReservation_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "NodeCapacityReservation_idempotencyKey_key" ON "NodeCapacityReservation"("idempotencyKey");
CREATE INDEX "NodeCapacityReservation_nodeId_status_expiresAt_idx" ON "NodeCapacityReservation"("nodeId", "status", "expiresAt");
ALTER TABLE "NodeCapacityReservation"
  ADD CONSTRAINT "NodeCapacityReservation_nodeId_fkey"
  FOREIGN KEY ("nodeId") REFERENCES "MinecraftNode"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
