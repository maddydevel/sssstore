# Operations Guide

## Health and readiness

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## Audit logs

Audit events are written to `audit_log_path` as JSON lines.

## Lifecycle and maintenance

- Stale multipart uploads are cleaned by the lifecycle worker.
- Run `sssstore doctor --scrub` to inspect metadata consistency.
- Run `sssstore doctor --scrub --repair` for basic repair actions.

## Replication beta

Set:
- `replication_mode=local_mirror`
- `replication_dir=/path/to/mirror`

This mirrors latest object state locally (beta behavior).
