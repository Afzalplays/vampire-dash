ALTER TABLE "MinecraftNode"
  ADD COLUMN "allocationStart" INTEGER NOT NULL DEFAULT 25565,
  ADD COLUMN "allocationEnd" INTEGER NOT NULL DEFAULT 25664;

ALTER TABLE "MinecraftNode"
  ADD CONSTRAINT "MinecraftNode_allocation_range_check"
  CHECK ("allocationStart" >= 1024 AND "allocationEnd" <= 65535 AND "allocationStart" <= "allocationEnd");

CREATE TABLE "NodePortAllocation" (
  "id" TEXT NOT NULL,
  "nodeId" TEXT NOT NULL,
  "serverId" TEXT NOT NULL,
  "port" INTEGER NOT NULL,
  "protocol" TEXT NOT NULL DEFAULT 'TCP',
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "NodePortAllocation_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "NodePortAllocation_serverId_key" ON "NodePortAllocation"("serverId");
CREATE UNIQUE INDEX "NodePortAllocation_nodeId_port_protocol_key" ON "NodePortAllocation"("nodeId", "port", "protocol");
CREATE INDEX "NodePortAllocation_nodeId_port_idx" ON "NodePortAllocation"("nodeId", "port");

ALTER TABLE "NodePortAllocation"
  ADD CONSTRAINT "NodePortAllocation_nodeId_fkey"
  FOREIGN KEY ("nodeId") REFERENCES "MinecraftNode"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

ALTER TABLE "NodePortAllocation"
  ADD CONSTRAINT "NodePortAllocation_serverId_fkey"
  FOREIGN KEY ("serverId") REFERENCES "MinecraftServer"("id") ON DELETE CASCADE ON UPDATE CASCADE;
