# API reference (starter)

Development base URL: http://127.0.0.1:4000/api. Cookie-authenticated endpoints require the browser to send credentials.

## Health
- GET /health — process health.
- GET /health/ready — database readiness.

## Authentication
- POST /auth/register — email, username and password; creates USER role only.
- POST /auth/login — email and password; sets an HttpOnly session cookie.
- GET /auth/me — current user.
- POST /auth/logout — revokes the current session.

## Billing (development only)
- POST /billing/orders — authenticated; amountPaise and provider (defaults to famgateway).
- POST /billing/orders/:orderId/verify — authenticated server-to-server verification; wallet credit is performed only after provider verification and amount/currency match.

## Admin
- GET /admin/users — admin only.
- GET /admin/payment-providers — admin only.
- POST /admin/users/:userId/wallet-adjustment — admin only; amountPaise and reason, recorded in the ledger and audit log.

## Important limitations
The webhook endpoint, raw-body signature verification, CSRF protection, MFA, OAuth, email verification and production security tests are not complete. Billing endpoints are not approved for live payments. Never expose this starter API publicly.
