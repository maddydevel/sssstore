# sssstore - Codebase Overview and Detailed Report

## Executive Summary

`sssstore` is a self-hosted, single-binary object storage service designed to provide S3-compatible behavior for practical development, edge deployments, and simple operational workflows. It implements a robust subset of the Amazon S3 HTTP API, emphasizing ease of use, strong operational visibility (metrics, JSON-line audit logging), and incremental feature scalability.

The architecture is built on a lightweight local filesystem backend for object data, paired with `bbolt` (an embedded key-value database) to durably manage metadata, versioning, and multipart upload state.

## Architecture & Modules

The codebase is logically structured into several core Go packages under `internal/` and a main entry point under `cmd/`.

### 1. Command-Line Interface (`cmd/sssstore/main.go`)
Provides the entry point with several subcommands:
- `init`: Initializes the data directory and generates a default `sssstore.json` configuration file.
- `server`: Runs the core HTTP server.
- `doctor`: Provides administrative commands like `--scrub` to detect metadata inconsistencies and `--repair` to fix orphan or missing metadata links.
- `user`: Allows operators to bootstrap IAM users (`create`) and assign policies (e.g., admin, read-write, read-only).

### 2. S3 API Handlers (`internal/s3api/`)
This module maps standard HTTP methods (GET, PUT, POST, DELETE, HEAD) to S3 operations.
- Supports bucket creation/deletion and object CRUD.
- Implements S3 features like `ListObjectsV2`, `GetBucketVersioning`, and `ListObjectVersions`.
- Routes Multipart upload specific endpoints (Initiate, UploadPart, Complete, Abort).
- Encodes S3-style XML responses for high compatibility with standard AWS SDKs.

### 3. Core Storage Engine (`internal/storage/`)
The heavy lifter of the codebase. It separates metadata and object backend implementations.
- **Object Backend:** Filesystem-backed storage (`newFSObjectBackend`) where raw bytes are written to disk. Objects are stored under `{data_dir}/{bucket}/objects/{key}`.
- **Metadata Store:** Originally filesystem JSON, now seamlessly migrated to **`bbolt`** (`newBboltMetadataStore`). It stores bucket creation times and object ETags.
- **Features:**
  - *Multipart Uploads*: Tracks active chunks and merges them upon completion.
  - *Versioning*: Handles version IDs by writing objects to a specific `versions` namespace when enabled on a bucket.
  - *Replication (Beta)*: Implements a basic `local_mirror` mode to continuously copy newly written objects to a secondary local directory.

### 4. Authentication & IAM (`internal/auth/`)
- Implements strict **AWS Signature Version 4** cryptographic request verification (`auth/sigv4.go`), ensuring clients correctly sign headers, URIs, and payloads using their access/secret keys.
- Enforces an IAM-like authorization policy middleware:
  - `admin`: Global access.
  - `read-write`: Prevented from creating/deleting buckets and altering bucket configurations.
  - `read-only`: Restricted entirely to HTTP GET/HEAD methods.

### 5. Server Runtime & Operations (`internal/server/`, `internal/audit/`, `internal/config/`)
- **Server:** Boots up the `net/http` listener. Exposes `/healthz`, `/readyz`, and `/metrics` (Prometheus-style metrics for requests/errors). Contains a background garbage collection worker to reap stale/abandoned multipart uploads based on a TTL (`MultipartMaxAgeHours`).
- **Audit Logging:** Emits structured JSON events (`internal/audit/audit.go`) for every authenticated HTTP request and system lifecycle event (e.g., garbage collection).
- **Config:** Strongly typed JSON unmarshaling (`sssstore.json`) covering TLS configurations, data directories, strict mode toggles, and replication paths.

## Current Project Status

Based on the `docs/CURRENT_STATUS.md` and codebase analysis, `sssstore` currently delivers a stable, single-node S3-compatible foundation.

### Capabilities Enabled
- Core S3 APIs (Buckets, Objects, ListObjectsV2).
- Complete Multipart Upload lifecycle.
- Bucket Versioning toggles.
- Strong AWS SigV4 Auth enforcement.
- Operational tooling (`healthz`, metrics, audit logs, doctor scrub utilities).

### Areas Currently Lacking
- Fine-grained Object-level Access Control Lists (ACLs).
- Presigned URL support (`X-Amz-Signature`).
- Server-Side Encryption (SSE).
- Asynchronous multi-node replication (only basic local mirror is present).
- Advanced routing and bucket policies (CORS, Rate Limiting, Static website hosting).

## Future Enhancement Plan

As outlined in the `docs/ENHANCEMENT_PLAN.md`, the roadmap is divided into two major phases:

### Phase 2: Production Hardening
The immediate next steps focus on bridging feature gaps to achieve high compatibility and security:
1. **Presigned URLs**: Upgrading the SigV4 authenticator to parse query-string parameters for time-bound upload/download links.
2. **Server-Side Encryption (SSE)**: Intercepting `PutObject`/`GetObject` streams to apply AES-256-GCM encryption on the fly, storing the nonces securely in `bbolt`.
3. **Canned ACLs & CORS**: Enabling bucket and object-level permissions (`public-read`, `private`) and browser Preflight `OPTIONS` rules for direct frontend integration.
4. **Enhanced Middlewares**: Adding explicit token-bucket Rate Limiting per AccessKey to prevent DoS attacks.

### Phase 3: Scale & Enterprise Features
Once the single-node engine is hardened, the focus shifts to distributed features:
1. **Asynchronous Target Replication**: Evolving the beta `local_mirror` into an active dispatcher capable of replicating state to secondary `sssstore` clusters or public cloud engines (GCS, AWS S3) for disaster recovery.
2. **Event Webhooks**: Dispatching S3-standard `ObjectCreated:Put` HTTP notifications to external CI/CD or indexing servers.
3. **OIDC/LDAP Integrations**: Permitting Single Sign-On, translating standard JWTs into scoped S3 temporary credentials.
4. **Multi-Tier Cold Storage**: Automatically offloading infrequently accessed files from local high-performance NVMe disks to cheaper external blob storage based on aging policies.

## Conclusion

The `sssstore` repository demonstrates a clean, modular Go architecture that balances simplicity with S3 spec compliance. The transition to `bbolt` for metadata management provides a solid transactional foundation, paving the way for the complex features outlined in Phase 2 and 3 of the enhancement plan.