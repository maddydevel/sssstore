# sssstore

`sssstore` is a single-binary, S3-compatible object storage server.

## Quick start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore user create --config ./sssstore.json --name ci --access-key ci-key --secret-key ci-secret
go run ./cmd/sssstore server --config ./sssstore.json
```

Server defaults to `:9000`.

## Supported APIs (Phase 2 baseline)

- Bucket: create, head, delete, list buckets
- Objects: put/get/head/delete
- Range GET support (`Range: bytes=start-end`)
- ListObjectsV2 (`?list-type=2`) with continuation token support
- Multipart upload:
  - `POST /{bucket}/{key}?uploads`
  - `PUT /{bucket}/{key}?partNumber=N&uploadId=...`
  - `POST /{bucket}/{key}?uploadId=...`
  - `DELETE /{bucket}/{key}?uploadId=...`
- ETag persistence for uploaded objects

## Security and auth

- S3 endpoints require an `Authorization` header with SigV4-like credential scope.
- Access keys are validated from `Credential=<access-key>/...` against:
  - `admin_access_key` in config
  - users created with `sssstore user create`
- `strict_mode` support in config:
  - requires non-default `admin_secret_key`
  - enforces TLS cert/key pair consistency

## Operability

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- JSON structured server logs (`log/slog`)
- JSON audit log file (`audit_log_path`)
- Lifecycle worker cleans stale multipart uploads older than `multipart_max_age_hours`

## CLI commands

- `sssstore init`
- `sssstore server`
- `sssstore doctor` (basic data directory checks)
- `sssstore user create` (basic admin bootstrap)

## Notes

This MVP focuses on local filesystem storage and baseline S3 path-style operations.
