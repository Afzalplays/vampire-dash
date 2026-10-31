# Plans and quotas

Plans are stored in a configurable database catalog. Administrators control price, billing duration and resource limits; no plan is enabled by default and no prices are seeded or assumed. Public clients can list enabled plans through `GET /api/plans`. Authenticated customers can inspect their currently effective plan entitlements through `GET /api/plans/me/entitlement`.

Plan limits are authoritative server-side values: RAM (MiB), disk (MiB), CPU percentage and concurrent server slots. A customer server-create operation must enforce the effective entitlement together with node capacity, location and port availability inside one database transaction. UI values are informational only.

The initial proposed free tier is 4 GB RAM, 10 GB storage, 200% CPU and one server slot, subject to operator configuration and actual node capacity. This is a proposal, not an enabled or seeded plan. Do not advertise a region until a healthy node is registered there. Paid 24/7 service must be an explicit entitlement. Expired accounts may be suspended according to published policy; customer data must not be silently deleted.

Plan purchase, payment-to-entitlement activation, entitlement renewal, server creation and Docker provisioning are not implemented by this catalog milestone. Do not treat wallet balance or a pending payment order as a plan entitlement.
