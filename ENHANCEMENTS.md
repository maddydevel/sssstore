# sssstore Additional Features and Enhancements

This document proposes the next wave of features after the initial MVP, focused on improving S3 compatibility, security, operability, performance, and ecosystem adoption.

## 1) S3 Compatibility Enhancements

## 1.1 API Surface Expansion (High Priority)
- **Bucket APIs**
  - `HeadBucket`
  - `GetBucketLocation`
  - `GetBucketVersioning` / `PutBucketVersioning`
  - `GetBucketLifecycleConfiguration` / `PutBucketLifecycleConfiguration`
  - `GetBucketCors` / `PutBucketCors`
- **Object APIs**
  - `CopyObject`
  - Conditional requests (`If-Match`, `If-None-Match`, `If-Modified-Since`)
  - Range GET support (`Range` header)
  - User metadata (`x-amz-meta-*`) persistence
- **Multipart Upload (critical for large objects)**
  - `CreateMultipartUpload`
  - `UploadPart`
  - `ListParts`
  - `CompleteMultipartUpload`
  - `AbortMultipartUpload`

## 1.2 Response/Behavior Fidelity (High Priority)
- Correct ETag behavior:
  - MD5-like ETag for single-part objects
  - Multipart ETag convention (`<md5>-<part-count>`)
- Stronger parity for error codes/messages with S3 conventions.
- Canonical XML formatting and namespace consistency.
- Path-style + optional virtual-hosted-style bucket addressing.

## 1.3 Compatibility Validation (High Priority)
- Add an interoperability test suite using AWS SDK v2 and `aws s3api`.
- Publish a compatibility matrix with:
  - Supported APIs
  - Partial support caveats
  - Not-yet-supported APIs

---

## 2) Security and Identity Enhancements

## 2.1 Authentication
- Full **SigV4** request verification (headers, canonical request, signed payload modes).
- Optional pre-signed URL support with expiration checks.
- Short-lived credentials (STS-lite) for CI jobs and temporary access.

## 2.2 Authorization
- IAM-like policy engine with:
  - Explicit deny precedence
  - Bucket and prefix-level resource matching
  - Action scoping (`s3:GetObject`, `s3:PutObject`, etc.)
- Bucket policy support for cross-account and public-read control (off by default).

## 2.3 Encryption and Secrets
- TLS configuration hardening and optional automatic cert reload.
- SSE support modes:
  - SSE-S3 (server-managed keys)
  - SSE-C (customer-provided key, optional)
- Pluggable KMS abstraction (file-based keyring first, Vault/KMS plugins later).

## 2.4 Audit and Compliance
- Structured append-only audit logs for:
  - Auth success/failure
  - Object read/write/delete
  - Policy and config changes
- Optional tamper-evident log chaining.
- Retention and object lock roadmap:
  - Governance mode first
  - Compliance mode later

---

## 3) Storage, Data Integrity, and Durability

## 3.1 Metadata and Indexing
- Move from ad-hoc filesystem-derived metadata to a dedicated metadata index:
  - Improves list performance
  - Enables efficient pagination and versioning
- Add schema versioning and migration support for future upgrades.

## 3.2 Integrity Controls
- Persist checksums (SHA-256 preferred) per object/version.
- Background scrubbing job with corruption detection and alerts.
- Optional write-ahead journal for crash consistency during object + metadata updates.

## 3.3 Versioning and Lifecycle
- Bucket-level versioning states:
  - Disabled / Enabled / Suspended
- Delete marker semantics compatible with S3.
- Lifecycle policies for expiration and non-current version cleanup.

## 3.4 Replication and Recovery
- Snapshot/restore tooling for metadata and config.
- Asynchronous replication mode (single-primary, standby).
- Disaster-recovery runbook automation checks.

---

## 4) Performance and Scalability Enhancements

## 4.1 Throughput and Latency
- Streaming IO everywhere (avoid full object buffering).
- Configurable worker pools and per-request resource limits.
- Read-ahead and zero-copy optimizations where practical.
- Fast-path for small objects and hot metadata caching.

## 4.2 Listing and Pagination
- Proper continuation token semantics for ListObjectsV2.
- Delimiter/prefix emulation for folder-like browsing.
- Optional pagination index to avoid full directory walks.

## 4.3 Deployment Modes
- **Standalone mode** (current evolution target)
- **Replicated mode** (active/passive)
- **Distributed mode** (longer-term):
  - Sharding strategy
  - Erasure coding
  - Placement policy abstraction

---

## 5) Operator Experience (CLI + Admin UX)

## 5.1 CLI Expansion
- `sssstore user` for credential lifecycle and key rotation.
- `sssstore bucket` for policy/versioning/lifecycle operations.
- `sssstore doctor` for health diagnostics and repair hints.
- `--json` output and stable exit codes for automation.

## 5.2 Embedded Admin Dashboard
- Overview: health, capacity, request/error rates.
- Buckets: policies, versioning, lifecycle.
- Access: users/keys/policies and recent auth failures.
- Audit explorer with filtering and export.

## 5.3 Observability
- `/metrics` (Prometheus format) and `/readyz` endpoint.
- Trace IDs in logs; request latency histograms.
- Alert recommendations for disk pressure and auth anomalies.

---

## 6) Developer and Community Enhancements

## 6.1 Developer Workflow
- Add Makefile tasks:
  - `make test`
  - `make lint`
  - `make integration`
  - `make build`
- Add static analysis (`go vet`, `staticcheck`) and race test profile.
- Add contract tests for API response fixtures.

## 6.2 Packaging and Releases
- Reproducible builds and signed release artifacts.
- Multi-arch binaries (linux/amd64, linux/arm64, darwin, windows).
- Minimal container image and example Kubernetes manifests.

## 6.3 Documentation
- Architecture docs:
  - request flow
  - data layout
  - consistency model
- Security hardening guide.
- Production runbook with backup/restore procedures.

---

## 7) Suggested Prioritized Delivery Plan

## Phase A: Compatibility + Safety (next 4-6 weeks)
1. HeadBucket, range GET, conditional headers.
2. SigV4 verification hardening.
3. ETag/checksum correctness.
4. ListObjectsV2 continuation tokens.
5. Compatibility matrix and SDK integration tests.

## Phase B: Large Object + Policy (next 6-8 weeks)
1. Multipart upload full flow.
2. IAM-like policy engine + bucket policies.
3. User/access key management CLI.
4. Audit logging baseline.

## Phase C: Durability + Operations (next 8-10 weeks)
1. Versioning + delete markers.
2. Lifecycle policies.
3. Metadata index and scrubber job.
4. `/metrics`, `/readyz`, structured observability.

## Phase D: Replication + UX (ongoing)
1. Replication beta.
2. Embedded dashboard MVP.
3. Release automation and community docs.

---

## 8) Quick Wins You Can Implement Immediately

1. Add `HeadBucket` endpoint.
2. Add object checksum + ETag persistence.
3. Implement `Range` in `GetObject`.
4. Add continuation tokens for `ListObjectsV2`.
5. Add `sssstore doctor` skeleton command.
6. Add `/readyz` and basic Prometheus metrics.
7. Add integration tests using AWS SDK Go v2.

These enhancements provide a practical path from the current MVP to a production-ready, S3-compatible storage system with strong security and operational reliability.
