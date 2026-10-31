# Admin guide

The starter API now includes server-side admin-only read access to users and enabled payment adapter IDs, plus a wallet adjustment endpoint. It relies on the database USER/ADMIN role. No public endpoint grants admin privileges; bootstrap an initial administrator only through a controlled, documented database operation after verifying the account owner.

Wallet adjustments require a reason, update the wallet and ledger in one database transaction, and write an audit record. Admin UI, node management, suspension controls, gateway credential configuration and broader audit search are not implemented yet. Protect admin credentials with MFA before production.
