# Admin guide

The starter API now includes server-side admin-only read access to users and enabled payment adapter IDs, plus a wallet adjustment endpoint. It relies on the database USER/ADMIN role. No public endpoint grants admin privileges; bootstrap an initial administrator only through a controlled, documented database operation after verifying the account owner.

Wallet adjustments require a reason, update the wallet and ledger in one database transaction, and write an audit record. Admin UI, node management, suspension controls, gateway credential configuration and broader audit search are not implemented yet. Protect admin credentials with MFA before production.


## Node registration API

Administrators can register a node with `POST /api/admin/nodes`. The endpoint must be HTTPS; credentials, query strings and fragments are rejected. Capacity values are bounded and duplicate endpoints are rejected. A cryptographically random agent bearer token is returned once, while only its keyed HMAC digest is stored. The node is created disabled and cannot be activated through this endpoint. Use `GET /api/admin/nodes` to list nodes without secrets and `POST /api/admin/nodes/:nodeId/disable` to disable one. Every registration and disable action is audit logged.

This is the registration foundation only: panel-to-agent mTLS enrollment, certificate issuance/rotation and node activation checks are not implemented yet. Keep the returned token in a secret manager and configure it on the node agent; do not paste it into tickets or commit it to Git.
