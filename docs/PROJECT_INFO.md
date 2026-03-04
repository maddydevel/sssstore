# About the Project

## What is sssstore?

sssstore is a self-hosted, single-binary object storage service that provides a practical subset of S3-compatible behavior for common development and operational workflows.

## Design goals

- Simple setup and operations
- Reasonable S3 API compatibility for common clients
- Strong operational visibility (health, metrics, logs, audit)
- Incremental feature delivery through roadmap phases

## Architecture overview

- **CLI** (`cmd/sssstore`) for initialization, server run, user bootstrap, and diagnostics
- **Config** (`internal/config`) for runtime and hardening settings
- **Server runtime** (`internal/server`) for HTTP serving, middleware, lifecycle tasks
- **S3 API handlers** (`internal/s3api`) implementing bucket/object endpoints
- **Storage** (`internal/storage`) filesystem-backed object/metadata, multipart, versioning, scrub
- **Auth/Audit** (`internal/auth`, `internal/audit`) for access-key checks and append-only event logs

## Current maturity

The project currently focuses on single-node filesystem-backed operation with practical S3-style APIs and operator tooling.
