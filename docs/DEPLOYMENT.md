# Deployment and HTTPS checklist

This project is not ready for production deployment. Production topology must use
HTTPS end-to-end for browser-facing services and mTLS for panel-to-node traffic.

## Public web and API

- Put the Next.js web app and Fastify API behind a maintained reverse proxy such as Nginx.
- Obtain certificates for every public hostname with Let's Encrypt/ACME or a trusted CA.
- Redirect HTTP port 80 to HTTPS; serve application traffic only on HTTPS.
- Enable TLS 1.2 or newer, automatic certificate renewal and renewal monitoring.
- Enable HSTS only after HTTPS works on every relevant subdomain; use a deliberate
  max-age and do not enable includeSubDomains/preload until all subdomains are HTTPS-ready.
- Set the exact HTTPS `WEB_ORIGIN` and `COOKIE_SECURE=true` in production.
- Keep the API bound to loopback/private network; do not publish its raw port.
- Trust proxy headers only from the known reverse proxy; configure proxy trust narrowly.
- Use secure, HttpOnly, SameSite cookies and implement CSRF defenses before production.

Example certificate issuance for an Nginx-hosted hostname:

```bash
sudo certbot --nginx -d panel.example.com
sudo certbot renew --dry-run
```

Replace the example hostname with a domain you control and ensure DNS points to the
server before issuance. Certbot configuration differs by operating system and proxy.

## Panel-to-node channel

- Use HTTPS with mutual TLS (mTLS); agent startup fails if TLS cert/key/client CA are absent.
- Issue per-node server certificates from a dedicated private CA and a panel API client
  certificate from the CA trusted by each agent.
- Validate hostnames/SANs and certificate expiry; never disable TLS verification.
- Add a unique random bearer token per node in addition to mTLS.
- Keep node-agent ports on private networking/VPN or firewall-allowlisted panel addresses.
- Never expose Docker socket or arbitrary host shell commands.

## Data and operations

- Use managed or hardened PostgreSQL and Redis with private network access and backups.
- Store secrets in a secret manager; rotate any exposed credential.
- Apply database migrations through a reviewed release process.
- Configure webhook signature validation and payment reconciliation before deposits.
- Test backup restoration, incident response, certificate renewal and rollback.
- Run dependency, container and application security scans.

Do not run a public installer or accept live payments until all release gates are satisfied.


## Automated installation target

See [INSTALLER-SPEC.md](INSTALLER-SPEC.md) for supported OS versions, panel/node profiles, dependency installation, TLS requirements, firewall/SSH preservation, backup/rollback behavior and release gates. A production installer must not be run from an unpinned branch or bypass certificate verification.
