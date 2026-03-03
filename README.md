# sssstore

`sssstore` is a single-binary, S3-compatible object storage server.

## Quick start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore user create --config ./sssstore.json --name ci --access-key ci-key --secret-key ci-secret
go run ./cmd/sssstore server --config ./sssstore.json
```

## Phase 3 baseline features

- Bucket/object APIs, `ListObjectsV2`, range GET, multipart upload.
- Bucket versioning controls:
  - `PUT /{bucket}?versioning`
  - `GET /{bucket}?versioning`
  - `GET /{bucket}?versions`
- Version-aware object operations via `versionId` query for GET/HEAD/DELETE.
- SigV4-style access-key auth scaffold (`Credential=<access-key>/...`).
- JSON structured logs + audit log file (`audit_log_path`).
- Lifecycle worker for stale multipart cleanup (`multipart_max_age_hours`).
- Local mirror replication beta (`replication_mode=local_mirror`, `replication_dir`).
- `sssstore doctor --scrub [--repair]` scrub/repair workflow.

## Security and operability config

- `strict_mode` enforces non-default admin secret and TLS cert/key pairing.
- TLS enabled when `tls_cert_file` + `tls_key_file` are both configured.
- Endpoints: `/healthz`, `/readyz`, `/metrics`.
