# sssstore

`sssstore` is a single-binary, S3-compatible object storage server.

## Quick start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore server --config ./sssstore.json
```

Server defaults to `:9000`.

## Supported APIs (current)

- Bucket: create, head, delete, list buckets
- Objects: put/get/head/delete
- Range GET support (`Range: bytes=start-end`)
- ListObjectsV2 (`?list-type=2`) with continuation token support
- ETag persistence for uploaded objects

## Ops endpoints

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## CLI commands

- `sssstore init`
- `sssstore server`
- `sssstore doctor` (basic data directory checks)

## Notes

This is an MVP implementation focused on local filesystem storage and baseline S3 path-style operations.

## Next steps

See [ENHANCEMENTS.md](./ENHANCEMENTS.md) for a prioritized feature enhancement backlog beyond the MVP.


## Foundations status (Phase 0)

- Project skeleton with module boundaries (`cmd/`, `internal/`).
- Config loader/initializer in `internal/config`.
- Basic HTTP server with `/healthz`, `/readyz`, and `/metrics`.
- Structured JSON request logs (`log/slog`).
- Local metadata/object store abstractions via `MetadataStore` and `ObjectBackend` interfaces in `internal/storage`.
