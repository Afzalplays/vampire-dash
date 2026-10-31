# Deployment checklist

This project is not ready for production deployment. Before exposing it to the internet:

- Deploy web and API behind HTTPS with an explicit trusted origin.
- Use managed or hardened PostgreSQL and Redis with private network access and backups.
- Store secrets in a secret manager; rotate any exposed credential.
- Enable secure cookies, CSRF protection, MFA and rate limits.
- Apply database migrations through a reviewed release process.
- Restrict API and node-agent network access; never expose Docker socket or host shell.
- Configure webhook signature validation and payment reconciliation before deposits.
- Test backup restoration, incident response and rollback.
- Run dependency, container and application security scans.

Do not run a public installer or accept live payments until all release gates are satisfied.
