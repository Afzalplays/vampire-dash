CREATE TYPE "MinecraftSoftwareType" AS ENUM ('VANILLA', 'PAPER', 'PURPUR', 'FABRIC', 'FORGE', 'NEOFORGE', 'VELOCITY', 'BUNGEECORD', 'BEDROCK');

CREATE TABLE "MinecraftSoftwareRelease" (
  "id" TEXT NOT NULL,
  "software" "MinecraftSoftwareType" NOT NULL,
  "minecraftVersion" TEXT NOT NULL,
  "softwareVersion" TEXT NOT NULL,
  "javaMajor" INTEGER NOT NULL,
  "dockerImage" TEXT NOT NULL,
  "downloadUrl" TEXT,
  "checksumSha256" TEXT,
  "recommended" BOOLEAN NOT NULL DEFAULT false,
  "enabled" BOOLEAN NOT NULL DEFAULT false,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "MinecraftSoftwareRelease_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "MinecraftSoftwareRelease_software_minecraftVersion_softwareVersion_key" ON "MinecraftSoftwareRelease"("software", "minecraftVersion", "softwareVersion");
CREATE INDEX "MinecraftSoftwareRelease_enabled_software_minecraftVersion_idx" ON "MinecraftSoftwareRelease"("enabled", "software", "minecraftVersion");
