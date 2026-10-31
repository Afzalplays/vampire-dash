# Admin guide

The starter API includes server-side admin-only read access to users and payment adapter IDs, plus a wallet adjustment endpoint. It relies on the database USER/ADMIN role. No public endpoint grants admin privileges; bootstrap an initial administrator only through a controlled, documented database operation after verifying the account owner.

Wallet adjustments require a reason, update the wallet and ledger in one database transaction, and write an audit record. Admin UI, suspension controls, gateway credential configuration and broader audit search are not implemented yet. Protect admin credentials with MFA before production.

## Node registration and health

Administrators can register a node with `POST /api/admin/nodes`. The endpoint must be HTTPS; credentials, query strings and fragments are rejected. Capacity values are bounded and duplicate endpoints are rejected. A cryptographically random agent bearer token is returned once, while the keyed HMAC digest and AES-256-GCM encrypted token are stored. New nodes remain disabled until secure mTLS enrollment and activation are implemented. Use `GET /api/admin/nodes` to list nodes and `POST /api/admin/nodes/:nodeId/check` to test telemetry connectivity. Keep the returned token in a secret manager; never paste it into tickets or commit it.

## Capacity reservations

An administrator can reserve capacity on an enabled, online node using `POST /api/admin/nodes/:nodeId/reservations`. The API serializes reservation attempts on the node row, subtracts configured memory/disk overhead, rejects overcommit, and requires a unique idempotency key. A reservation expires after 15 minutes unless consumed or explicitly released. Use the release endpoint to free a reservation early. Reservations are groundwork for server provisioning and are not yet connected to customer server creation or Docker operations.

Node telemetry reports host-level available resources. It is not a replacement for configured allocatable capacity, resource quotas or a reservation.

A dedicated reservation-expiry worker expires stale holds every 30 seconds and reconciles the node's cached reserved-capacity counters. The admission path also excludes expired holds directly, so a delayed worker does not permit overcommit.
