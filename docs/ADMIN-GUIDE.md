# Admin guide

Admin dashboard and management APIs are not implemented yet. The database has a USER/ADMIN role enum and the API includes a server-side admin authorization helper, but no public route grants admin access.

Planned administration: user lifecycle, node registration, server quotas, slots, payment-provider configuration, wallet adjustments, suspension policy and audit log review. Every privileged action must record actor, target, reason and timestamp. Never grant admin through registration or browser-supplied role data.
