# FamGateway integration status

FamGateway is the initial/default provider adapter. The adapter can create provider orders and perform server-to-server order verification. The API exposes authenticated payment-order creation and a manual verification endpoint for development.

## Required before live payments
- Implement and test the provider webhook route with raw-body HMAC-SHA256 verification and timing-safe comparison.
- Confirm the exact current provider response schema for amount, currency, paid status and transaction ID.
- Add webhook event idempotency, transaction reconciliation, refund workflow and alerting.
- Add CSRF protection to cookie-authenticated write endpoints.
- Add integration tests for wrong amount, currency, invalid signature, duplicate callbacks, timeout and race conditions.
- Perform a security review before accepting real money.

Never treat browser redirects or screenshots as payment proof. Wallet credits must follow server-side verification and be protected by a unique ledger reference. FamGateway credentials remain server-side only. Do not enable live deposits until all required checks pass.

## Autopay
A one-time QR/payment order is not recurring UPI autopay. Do not advertise recurring mandates unless the provider documents mandate creation, authorization and recurring debit support.
