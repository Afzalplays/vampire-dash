# Pluggable payment gateways

FamGateway is the initial default provider. The billing system is designed around independent provider adapters so other supported gateways can be added later. The planned admin UI will let authorized administrators configure enabled providers and select the default.

## Adapter responsibilities
- Create a provider payment order.
- Verify payment server-to-server.
- Validate webhook signatures using the provider's documented method.
- Normalize statuses and transaction IDs.
- Support refunds only where the provider offers a documented API.

## Security requirements
1. Keep API keys server-side in environment variables or a secrets manager. Never expose them to browser code or commit them.
2. Create a pending internal order before calling a provider.
3. Verify amount, currency, internal order ID, provider transaction and paid status on the backend.
4. Validate webhook signatures against the raw request body and use timing-safe comparison.
5. Process webhooks idempotently and credit the wallet once, in a database transaction.
6. Redirects and browser polling are status display only, never proof of payment.
7. Do not advertise recurring autopay unless the provider documents mandate APIs and customer authorization.

This is design documentation. Live billing is not implemented yet.
