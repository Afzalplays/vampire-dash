# Email verification and notifications

Vampire Dash supports a 6-digit email verification code during registration. Codes are random, stored as SHA-256 digests, expire after 10 minutes, and are limited to five verification attempts. Registration does not issue a session until the address is verified. Resend and verify endpoints are rate limited.

## SMTP configuration

Configure SMTP on the API host only. Never expose SMTP credentials to the browser or commit them.

```env
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_SECURE=false
SMTP_USER=your-smtp-user
SMTP_PASSWORD=your-smtp-password
SMTP_FROM=VampireHosting <no-reply@example.com>
```

For implicit TLS (commonly port 465), set SMTP_SECURE=true. Use a real provider mailbox and configure SPF, DKIM and DMARC for reliable delivery.

## Notification events

Templates are available for server created, expiry in 7 days and 1 day, suspended/restored, payment received/failed, password changed and security alerts. User preferences are available through `GET/PATCH /api/notifications/preferences`.

## Expiry worker

Run `npm run worker:expiry --workspace @vampire-dash/api` as a separate supervised process. It scans hourly for servers with an `expiresAt` timestamp, sends one 7-day or 1-day reminder per server/event, and uses a unique dedupe key to avoid duplicate reminders. The worker starts with an immediate scan. Ensure the process is supervised by systemd, Docker restart policy or another process manager.

The notification service checks each user's email and event preferences. Delivery status and errors are recorded in the database. SMTP must be configured for delivery.

## Remaining production work

The worker is an initial scheduler, not a durable queue. Add bounded retries/backoff, bounce handling, delivery monitoring, lifecycle event wiring for server create/suspend/restore, and integration tests before relying on email for critical customer operations. The worker must be run in production; it is not started automatically by the web/API process.
