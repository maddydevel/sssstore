# sssstore

`sssstore` is a single-binary, S3-compatible object storage server.

## Quick start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore server --config ./sssstore.json
```

Server defaults to `:9000`.

## Supported APIs (current)

- Bucket: create, delete, list buckets
- Objects: put/get/head/delete
- ListObjectsV2 (`?list-type=2`)
- Health endpoint: `GET /healthz`

## Notes

This is an MVP implementation focused on local filesystem storage and baseline S3 path-style operations.
