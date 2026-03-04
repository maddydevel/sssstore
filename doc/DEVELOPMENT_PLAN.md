# sssstore — Single-Binary S3-Compatible Storage

## 1) Product Vision and Scope

**Goal:** build `sssstore`, an open-source, single-binary object storage server that is S3-compatible enough for common SDKs/tools while staying easy to deploy and operate.

**Non-goals for v1:**
- Full AWS parity for every edge-case API behavior.
- Multi-region replication orchestration in core binary.
- Extremely large-cluster distributed metadata consensus in the first release.

**North-star principles:**
1. **Single binary, minimal dependencies.**
2. **Strong defaults for security and durability.**
3. **Operational simplicity** (small teams can run it).
4. **Progressive capability** from single-node to small cluster.

---

## 2) Target Audience and Use Cases

### Primary users
- Platform/devops teams needing S3-compatible storage on-prem or edge.
- Developers building against S3 APIs for local/testing environments.
- SMBs/startups that need low-complexity private object storage.
- Regulated environments requiring self-hosting and auditable access.

### Common use cases
- Artifact and backup storage.
- Static asset/object delivery for applications.
- Data lake landing zone for logs/events.
- Air-gapped or sovereign deployment where managed cloud is unavailable.

### Personas and expectations
- **Operator persona:** wants simple deploy, clear metrics, easy upgrades.
- **Developer persona:** wants predictable S3 behavior + solid CLI.
- **Security persona:** wants identity controls, encryption, immutable logs.

---

## 3) Core Feature Set

## 3.1 S3 Compatibility Baseline (v1)
Implement high-value APIs first:
- `ListBuckets`, `CreateBucket`, `DeleteBucket`, `HeadBucket`.
- `PutObject`, `GetObject`, `HeadObject`, `DeleteObject`.
- Multipart upload lifecycle (`CreateMultipartUpload`, `UploadPart`, `CompleteMultipartUpload`, `AbortMultipartUpload`).
- `ListObjectsV2`, `ListObjectVersions` (if versioning enabled).
- `CopyObject` (server-side copy).

Compatibility goals:
- Works with AWS SDKs (Go/Java/Python), `aws s3`, and `s3cmd` for supported operations.
- Strict request canonicalization/signature verification for SigV4.

## 3.2 Bucket Management
- Bucket create/delete/list with validation and namespace constraints.
- Bucket-level settings:
  - Versioning state (`Disabled`, `Enabled`, optional `Suspended` behavior).
  - Object lock mode (future phase).
  - Lifecycle policy (initially expiration for prefix/tag filters).
  - CORS policy for browser-based clients.

## 3.3 Object Versioning
- Immutable versions with generated version IDs.
- Delete marker semantics consistent with S3 behavior.
- Read path resolves latest non-delete-marker unless explicit version requested.
- Retention support roadmap:
  - Phase 1: soft retention timestamps.
  - Phase 2: governance/compliance lock semantics.

## 3.4 Access Control
- IAM-like policy engine (JSON policy docs), scoped to:
  - account/user,
  - bucket,
  - object prefix.
- Access keys + secret keys with rotation support.
- Service accounts and optional STS-like temporary credentials in later phases.
- Optional public-read bucket policy but disabled by default.

## 3.5 Durability and Integrity
- Strong integrity checks with content hash (ETag semantics for single-part; documented behavior for multipart).
- Write-ahead metadata journal for crash recovery.
- Background scrubber:
  - verifies object checksums,
  - quarantines corrupt objects,
  - emits repair alerts.
- Configurable replication profile:
  - single-disk (dev),
  - mirrored (RAID-like pair),
  - erasure coding (cluster mode phase).

---

## 4) User Interfaces

## 4.1 Command-Line Interface (`sssstore`)
CLI structure:
- `sssstore server` — run API + optional web UI.
- `sssstore init` — bootstrap config, keys, storage paths.
- `sssstore admin` — operator actions.
- `sssstore user` — user/access-key management.
- `sssstore bucket` — bucket operations and policies.
- `sssstore doctor` — diagnostics and repair suggestions.

Example commands:
- `sssstore init --config /etc/sssstore/config.yaml --data /var/lib/sssstore`
- `sssstore server --config /etc/sssstore/config.yaml`
- `sssstore user create --name ci-bot --policy readonly-artifacts`
- `sssstore bucket policy set mybucket --file policy.json`

Usability considerations:
- Human-readable table output + `--json` for automation.
- `--dry-run` for destructive admin commands.
- Exit codes documented for scripting.
- `sssstore completion` for shell completion.

## 4.2 Web Dashboard
Dashboard is embedded in binary (static assets via `embed`).

Key screens:
1. **Overview:** service health, capacity, request rates, error rates.
2. **Buckets:** create/delete, policy/versioning/lifecycle management.
3. **Objects (lightweight browser):** list/search/download/delete by prefix.
4. **Identity & Access:** users, keys, policies, recent auth failures.
5. **Audit & Events:** filterable logs for security and compliance.
6. **Settings:** TLS certs, retention, replication, notifications.

UX design principles:
- Operator-first: low clicks for common tasks.
- Safe interactions: explicit confirm for destructive actions.
- Accessibility: keyboard support, readable contrast, clear status states.
- Progressive disclosure: basic mode by default, advanced panels expandable.

---

## 5) Technology Stack (Go)

## 5.1 Why Go
- Excellent concurrency model for high request fan-in/out.
- Strong standard library for networking, crypto, and HTTP.
- Produces static binaries suitable for the single-binary requirement.
- Mature profiling/tooling (`pprof`, `trace`) improves performance tuning.

## 5.2 Recommended Libraries
- HTTP router: `chi` or `httprouter` (low overhead, maintainable middleware).
- S3 protocol compatibility helpers: implement core in-house + selective reuse of battle-tested parsers.
- Storage engine metadata:
  - `bbolt` for simple local metadata (v1 single-node), or
  - `badger` for higher write throughput needs.
- Logging: `zap` (structured, fast).
- Metrics: `prometheus/client_golang`.
- Auth/JWT where needed: `golang-jwt/jwt` (if tokenized dashboard sessions).
- Config: `koanf` or `viper` + strict schema validation layer.

## 5.3 Maintainability implications
- Keep adapters cleanly separated:
  - `api/` (S3 handlers),
  - `authz/` (policy engine),
  - `storage/` (pluggable backend interfaces),
  - `meta/` (bucket/object metadata),
  - `cmd/` (CLI entrypoints).
- Enforce API contracts with table-driven tests and compatibility fixtures.
- Avoid framework lock-in; prefer standard library + minimal dependencies.

---

## 6) Security Architecture

## 6.1 Authentication
- AWS SigV4 for S3 API requests.
- Static access keys for v1 with optional key expiration.
- Optional OIDC login for dashboard (phase 2), fallback local admin account.
- Rate limiting and brute-force protection for auth endpoints.

## 6.2 Authorization
- Policy engine semantics modeled after AWS IAM subset:
  - `Effect`, `Action`, `Resource`, `Condition`.
- Policy evaluation order: explicit deny > allow > implicit deny.
- Bucket policy + identity policy combination documented and deterministic.

## 6.3 Encryption
- In transit: TLS 1.2+ mandatory for non-local environments.
- At rest:
  - default SSE with per-object data keys,
  - KEK from local KMS plugin interface (file/HSM/Vault-backed).
- Key rotation process with lazy re-encryption background jobs.

## 6.4 Audit Logging
- Append-only structured audit log events:
  - auth success/failure,
  - object read/write/delete,
  - policy/admin changes.
- Tamper-evident chaining (hash-link events) for compliance mode.
- Export to SIEM via syslog/HTTP webhook.

## 6.5 Secure Defaults Checklist
- No anonymous access unless explicitly enabled.
- Dashboard disabled or localhost-only by default.
- Secrets never logged.
- `--strict` mode blocks weak TLS/ciphers and insecure config.

---

## 7) Scalability and Performance

## 7.1 Deployment Modes
1. **Standalone mode (v1):**
   - single binary + local disk(s),
   - ideal for edge/dev/small production.
2. **Replicated mode (v1.5):**
   - active/passive with log shipping and failover.
3. **Distributed mode (v2):**
   - sharded metadata + erasure-coded object placement.

## 7.2 Storage Backend Strategy
Define `ObjectBackend` and `MetadataStore` interfaces:
- Local FS backend (baseline).
- Optional block/object plugin backends (future).
- Metadata backend starts embedded, can graduate to external DB for larger scale.

## 7.3 Performance Patterns
- Streaming uploads/downloads (no full-buffer object load).
- Multipart concurrency tuning per bucket/client.
- Read-ahead and small-object packing optimizations.
- Background compaction + lifecycle jobs with bounded resource usage.
- Built-in profiling endpoints (admin-only).

## 7.4 SLO guidance
- Availability target by mode:
  - standalone: best effort / single-node constraints.
  - replicated: 99.9% with tested failover runbooks.
- Latency targets:
  - p50 low-ms for metadata ops,
  - throughput-first for large-object transfers.

---

## 8) Data Model and Durability Blueprint

## 8.1 Object layout
- Data blobs stored content-addressed or bucket/prefix-addressed (choose and document).
- Metadata record includes:
  - bucket, key, version ID,
  - size, checksum, content-type,
  - encryption metadata,
  - timestamps and retention flags.

## 8.2 Consistency model
- v1 target: read-after-write consistency for new objects in standalone mode.
- Overwrite/delete consistency clearly documented (avoid surprising eventual semantics unless unavoidable).

## 8.3 Recovery model
- Startup performs journal replay and metadata integrity checks.
- `sssstore doctor` can detect and optionally repair orphaned blobs or stale metadata.
- Snapshot + restore tooling for metadata and policy state.

---

## 9) API and Compatibility Strategy

- Publish explicit compatibility matrix:
  - Supported S3 operations,
  - Known deviations,
  - Unsupported advanced features.
- Use golden interoperability tests against AWS SDK behavior for accepted APIs.
- Version API behavior changes and provide migration notes.

---

## 10) Roadmap and Milestones

## Phase 0 — Foundations (Weeks 1–4)
- Project skeleton, module boundaries, config loader.
- Basic HTTP server, health endpoints, structured logs, metrics.
- Local metadata and object store abstractions.

## Phase 1 — MVP S3 Core (Weeks 5–10)
- Bucket + object CRUD.
- SigV4 auth and basic policy checks.
- Multipart uploads.
- CLI for bootstrap and basic admin.

## Phase 2 — Operability & Security (Weeks 11–16)
- Dashboard alpha.
- Audit logs + lifecycle jobs.
- TLS hardening, key rotation tooling.
- Compatibility test suite with common SDKs.

## Phase 3 — Durability & Scale Extensions (Weeks 17–24)
- Versioning GA.
- Replication mode beta.
- Scrubbing/repair workflows.
- Performance tuning and benchmark publication.

## Phase 4 — Ecosystem and Hardening (ongoing)
- Helm chart / container image.
- Documentation expansion and tutorials.
- Community governance, issue triage, release cadence.

---

## 11) Deployment, Packaging, and Maintenance

## 11.1 Single-binary packaging
- Produce reproducible builds with `-trimpath` and version metadata.
- Provide binaries for Linux/macOS/Windows + checksums/signatures.
- Optional minimal container image for Kubernetes users.

## 11.2 Upgrades
- Backward-compatible config migration tool (`sssstore migrate-config`).
- Metadata schema versioning with online migration where possible.
- Rolling restart guidance for replicated/distributed modes.

## 11.3 Observability and operations
- `/metrics`, `/healthz`, `/readyz` endpoints.
- Alerting playbooks for disk pressure, auth anomalies, replication lag.
- Periodic chaos/recovery drills documented.

## 11.4 Community strategy
- Contributor guide, code of conduct, architecture decision records.
- “Good first issue” labeling and monthly office-hours/community sync.
- Public roadmap and release notes discipline.

---

## 12) Suggested Repository Structure

```text
sssstore/
  cmd/sssstore/
  internal/
    api/
    authn/
    authz/
    storage/
    metadata/
    lifecycle/
    replication/
    audit/
    observability/
  webui/                # source UI assets
  doc/
  test/
    compat/
    integration/
    load/
```

---

## 13) Risk Register and Mitigation

1. **S3 edge-case incompatibilities**
   - Mitigation: compatibility matrix + golden tests + explicit non-goals.
2. **Security misconfiguration by operators**
   - Mitigation: secure-by-default config and startup warnings/errors.
3. **Metadata bottlenecks at scale**
   - Mitigation: abstraction to swap metadata backend and early benchmarks.
4. **Single-binary scope creep**
   - Mitigation: strict module boundaries and phased feature gates.

---

## 14) Definition of Done (per feature)

A feature is done when:
- API behavior documented.
- Unit + integration tests added.
- Metrics and audit events included.
- Security review checklist passed.
- CLI and/or dashboard workflow implemented.
- Upgrade/backward compatibility impact assessed.

---

## 15) Immediate Next Steps (Actionable)

1. Initialize Go module and base project structure.
2. Implement config + logger + health endpoints.
3. Build metadata/object interfaces with local FS backend.
4. Add SigV4 request parser and auth middleware scaffold.
5. Implement `CreateBucket`, `PutObject`, `GetObject`, `ListObjectsV2`.
6. Add CLI bootstrap commands (`init`, `server`, `user create`).
7. Stand up interoperability tests with AWS SDK Go v2.

This roadmap yields a pragmatic path to a production-capable, open-source, single-binary S3-compatible system while preserving maintainability and security from day one.
