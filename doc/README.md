# sssstore

`sssstore` is an open-source, single-binary, S3-compatible object storage server designed for easy self-hosting, local development, edge deployments, and private cloud environments.

This repository includes the runnable server, CLI tooling, filesystem-backed storage engine, and project documentation needed to deploy and operate sssstore.

## Documentation Index

- [Installation Guide](./INSTALLATION.md)
- [Project Information](./PROJECT_INFO.md)
- [Best Use Cases](./USE_CASES.md)
- [Examples](./EXAMPLES.md)
- [User Guide](./USER_GUIDE.md)
- [Admin Guide](./ADMIN_GUIDE.md)
- [API Guide](./API_GUIDE.md)
- [Operations Guide](./OPERATIONS.md)
- [Compatibility Matrix](./COMPATIBILITY.md)
- [Security Policy](./SECURITY.md)
- [Contributing](./CONTRIBUTING.md)

## Quick Start

```bash
go run ./cmd/sssstore init --config ./sssstore.json --data ./data
go run ./cmd/sssstore user create --config ./sssstore.json --name ci --access-key ci-key --secret-key ci-secret
go run ./cmd/sssstore server --config ./sssstore.json
```

## Current Highlights

- Single binary deployment model
- S3-style bucket/object APIs
- Multipart upload support
- Bucket versioning support
- Structured logs, metrics, health/readiness endpoints
- Audit logging and scrub/repair tooling

For full details, use the documentation index above.
