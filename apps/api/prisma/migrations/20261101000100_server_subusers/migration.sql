CREATE TABLE "MinecraftServerSubuser" (
  "id" TEXT NOT NULL,
  "serverId" TEXT NOT NULL,
  "userId" TEXT NOT NULL,
  "viewConsole" BOOLEAN NOT NULL DEFAULT true,
  "power" BOOLEAN NOT NULL DEFAULT false,
  "manageFiles" BOOLEAN NOT NULL DEFAULT false,
  "manageBackups" BOOLEAN NOT NULL DEFAULT false,
  "manageSettings" BOOLEAN NOT NULL DEFAULT false,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "MinecraftServerSubuser_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "MinecraftServerSubuser_serverId_userId_key" ON "MinecraftServerSubuser"("serverId", "userId");
CREATE INDEX "MinecraftServerSubuser_userId_createdAt_idx" ON "MinecraftServerSubuser"("userId", "createdAt");
ALTER TABLE "MinecraftServerSubuser" ADD CONSTRAINT "MinecraftServerSubuser_serverId_fkey"
  FOREIGN KEY ("serverId") REFERENCES "MinecraftServer"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "MinecraftServerSubuser" ADD CONSTRAINT "MinecraftServerSubuser_userId_fkey"
  FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE CASCADE ON UPDATE CASCADE;
