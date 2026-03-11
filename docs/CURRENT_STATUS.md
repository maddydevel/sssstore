# Current Project Status (Snapshot)

This status summarizes where `sssstore` stands today by mapping the current documented capabilities to the roadmap phases.

## Overall maturity

- **Current stage:** single-node, filesystem-backed S3-compatible service focused on practical APIs and operator tooling.
- **Operational posture:** health/readiness/metrics endpoints, JSON-line audit logs, lifecycle cleanup for stale multipart uploads, and scrub/repair tooling.

## What is currently implemented (core)

- **S3-style APIs:** buckets, objects, ListObjectsV2, multipart upload flow.
- **Versioning:** bucket versioning APIs and list versions behavior are available.
- **Runtime model:** single-binary server + CLI with config, auth, audit, and storage modules.

## Phase 2 (Production Hardening) status

### Already available or partially available

- **Object versioning:** implemented at bucket/object API level.
- **Lifecycle worker:** currently used for stale multipart cleanup.
- **Audit logging:** implemented as JSON lines and integrated into request/lifecycle events.

### Not yet implemented (per roadmap/compatibility)

- **ACL APIs** (canned ACL policies).
- **SSE (AES-256-GCM) object encryption**.
- **Presigned URL query-signature flow** (`X-Amz-Signature`/`X-Amz-Expires`).
- **Object tagging APIs**.
- **CORS policy enforcement for browser preflight**.
- **Explicit per-key/IP rate limiting middleware**.

## Phase 3 (Scale & Enterprise) status

- **Replication:** local mirror beta exists, but enterprise async multi-target replication is not yet present.
- **Not yet implemented:** quotas, S3 event webhooks, static website hosting, OIDC/LDAP SSO flows, and multi-tier hot/warm/cold routing.

## Suggested near-term focus

1. Complete Phase 2 security/usability gaps first (ACLs, presigned URLs, SSE, CORS).
2. Add request throttling and richer audit export integrations.
3. Evolve replication from local mirror beta to durable async target replication.
