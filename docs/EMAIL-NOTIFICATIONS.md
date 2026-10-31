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

The email notification service includes templates for:
- Server created
- Server expiry in 7 days and 1 day
- Server suspended or restored
- Payment received or failed
- Password changed and security alerts

Users can read and update email notification preferences at `GET/PATCH /api/notifications/preferences`. Email delivery is performed server-side.

## Remaining production work

The event templates and send service are starter infrastructure. Server lifecycle, billing and security code must call the notification service at the relevant transaction boundary. A durable retrying queue, scheduled expiry scanner (7-day/1-day reminders), delivery monitoring, bounce handling and integration tests are still required before relying on these messages for customer operations.
