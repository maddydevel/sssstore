# sssstore

`sssstore` is a single-binary, S3-compatible object storage server.

## Quick start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore user create --config ./sssstore.json --name ci --access-key ci-key --secret-key ci-secret
go run ./cmd/sssstore server --config ./sssstore.json
```

Server defaults to `:9000`.

## Supported APIs (Phase 1 MVP core)

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

## Authentication (SigV4 scaffold)

S3 endpoints require `Authorization` header in SigV4 format and validate the access key from `Credential=<access-key>/...`.

Supported keys:
- `admin_access_key` from config
- users created with `sssstore user create`

## Ops endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## CLI commands

- `sssstore init`
- `sssstore server`
- `sssstore doctor` (basic data directory checks)
- `sssstore user create` (basic admin bootstrap)

## Notes

This MVP focuses on local filesystem storage and baseline S3 path-style operations.
