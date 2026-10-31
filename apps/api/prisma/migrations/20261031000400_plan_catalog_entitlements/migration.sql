CREATE TYPE "PlanEntitlementStatus" AS ENUM ('ACTIVE', 'EXPIRED', 'CANCELLED');

CREATE TABLE "Plan" (
  "id" TEXT NOT NULL,
  "slug" TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "description" TEXT NOT NULL DEFAULT '',
  "pricePaise" BIGINT NOT NULL,
  "currency" TEXT NOT NULL DEFAULT 'INR',
  "billingIntervalDays" INTEGER NOT NULL,
  "memoryMb" INTEGER NOT NULL,
  "diskMb" INTEGER NOT NULL,
  "cpuPercent" INTEGER NOT NULL,
  "serverSlots" INTEGER NOT NULL,
  "enabled" BOOLEAN NOT NULL DEFAULT false,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "Plan_pkey" PRIMARY KEY ("id")
);

CREATE TABLE "UserPlanEntitlement" (
  "id" TEXT NOT NULL,
  "userId" TEXT NOT NULL,
  "planId" TEXT NOT NULL,
  "status" "PlanEntitlementStatus" NOT NULL DEFAULT 'ACTIVE',
  "startsAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expiresAt" TIMESTAMP(3) NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "UserPlanEntitlement_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "Plan_slug_key" ON "Plan"("slug");
CREATE INDEX "Plan_enabled_pricePaise_idx" ON "Plan"("enabled", "pricePaise");
CREATE INDEX "UserPlanEntitlement_userId_status_expiresAt_idx" ON "UserPlanEntitlement"("userId", "status", "expiresAt");
CREATE INDEX "UserPlanEntitlement_planId_status_idx" ON "UserPlanEntitlement"("planId", "status");

ALTER TABLE "UserPlanEntitlement" ADD CONSTRAINT "UserPlanEntitlement_userId_fkey"
FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
ALTER TABLE "UserPlanEntitlement" ADD CONSTRAINT "UserPlanEntitlement_planId_fkey"
FOREIGN KEY ("planId") REFERENCES "Plan"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
