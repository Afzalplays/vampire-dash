CREATE TYPE "MinecraftArtifactKind" AS ENUM ('PLUGIN', 'MOD', 'MODPACK');
CREATE TYPE "MinecraftArtifactProvider" AS ENUM ('MODRINTH', 'CURSEFORGE');

CREATE TABLE "MinecraftArtifactProject" (
  "id" TEXT NOT NULL,
  "kind" "MinecraftArtifactKind" NOT NULL,
  "provider" "MinecraftArtifactProvider" NOT NULL,
  "providerProjectId" TEXT NOT NULL,
  "slug" TEXT NOT NULL,
  "title" TEXT NOT NULL,
  "summary" TEXT NOT NULL DEFAULT '',
  "projectUrl" TEXT NOT NULL,
  "sourceUrl" TEXT,
  "licenseId" TEXT,
  "supportedSoftware" "MinecraftSoftwareType"[],
  "supportedGameVersions" TEXT[],
  "loaders" TEXT[],
  "enabled" BOOLEAN NOT NULL DEFAULT false,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "MinecraftArtifactProject_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "MinecraftArtifactProject_provider_providerProjectId_key" ON "MinecraftArtifactProject"("provider", "providerProjectId");
CREATE INDEX "MinecraftArtifactProject_enabled_kind_provider_idx" ON "MinecraftArtifactProject"("enabled", "kind", "provider");

CREATE TABLE "MinecraftArtifactFile" (
  "id" TEXT NOT NULL,
  "projectId" TEXT NOT NULL,
  "providerFileId" TEXT NOT NULL,
  "versionNumber" TEXT NOT NULL,
  "downloadUrl" TEXT NOT NULL,
  "sha256" TEXT,
  "sizeBytes" BIGINT,
  "gameVersions" TEXT[],
  "loaders" TEXT[],
  "enabled" BOOLEAN NOT NULL DEFAULT false,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "MinecraftArtifactFile_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "MinecraftArtifactFile_projectId_fkey" FOREIGN KEY ("projectId") REFERENCES "MinecraftArtifactProject"("id") ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX "MinecraftArtifactFile_projectId_providerFileId_key" ON "MinecraftArtifactFile"("projectId", "providerFileId");
CREATE INDEX "MinecraftArtifactFile_enabled_projectId_idx" ON "MinecraftArtifactFile"("enabled", "projectId");
