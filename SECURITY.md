# Security Policy

## Reporting vulnerabilities

Please report vulnerabilities privately to project maintainers.
Do not open public issues for sensitive security findings.

## Security hardening guidance

- Enable `strict_mode` in production.
- Set a non-default `admin_secret_key`.
- Configure TLS via `tls_cert_file` and `tls_key_file`.
- Rotate access keys and review audit logs regularly.
