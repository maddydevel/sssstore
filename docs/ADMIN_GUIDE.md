# Admin Guide

## Server bootstrap

1. Initialize config/data:
   - `sssstore init --config ./sssstore.json --data ./data`
2. Create at least one non-admin user key:
   - `sssstore user create --config ./sssstore.json --name app --access-key app-key --secret-key app-secret`
3. Start server:
   - `sssstore server --config ./sssstore.json`

## Production hardening checklist

- Enable `strict_mode` in config
- Set strong non-default `admin_secret_key`
- Configure TLS (`tls_cert_file`, `tls_key_file`)
- Set explicit `audit_log_path`
- Configure replication beta only with validated storage paths

## Monitoring

- `/healthz` for liveness
- `/readyz` for readiness
- `/metrics` for scraping counters
- Audit log JSON lines for security and operation event trails

## Maintenance

- Run `sssstore doctor --scrub` regularly
- Run `sssstore doctor --scrub --repair` for basic metadata repair
- Validate stale multipart cleanup settings (`multipart_max_age_hours`)

## Replication beta

`local_mirror` mode mirrors latest object state to `replication_dir`. Treat this as beta and validate recovery procedures before production usage.
