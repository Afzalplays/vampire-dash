# Admin guide

Administrators can inspect users, adjust wallet balances with an audited ledger entry, register nodes, check node health and manage node capacity reservations. Node registration returns its bearer token once and leaves the node disabled until mTLS enrollment is implemented.

Capacity reservations are admin-only groundwork for provisioning. They require an enabled, online node, serialize capacity admission on the node row, subtract configured overhead, enforce idempotency and expire after 15 minutes. A dedicated worker expires stale holds and reconciles cached reserved-capacity counters.

The plan catalog is database-backed. Add plan-management controls only after defining the admin workflow; plans are disabled by default and no prices are seeded. Customer-facing enabled plans are read-only through `GET /api/plans`. Entitlements currently have no self-service purchase path and must not be manually granted without an operator policy.
