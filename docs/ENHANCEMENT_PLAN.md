# sssstore — Future Feature & Enhancement Plan

Based on the roadmap of highly scalable S3-compatible storage engines (like the *sangraha* blueprint), the following features are strategically recommended for the next phases of `sssstore` development. They are logically grouped into Production Hardening and Enterprise Scalability.

---

## Phase 2: Production Hardening

### 1. Object Versioning & Access Control Lists (ACLs)
- **Per-Bucket Versioning**: Implement REST API toggles for `enabled`, `suspended`, or `disabled` on individual buckets.
- **Version IDs**: Assign ULIDs to objects when versioning is active so modifications are stacked rather than overwritten.
- **Delete Markers**: Introduce `DeleteMarkers` to hide files instead of permanently purging them, making deletions reversible.
- **Canned ACLs**: Support `private`, `public-read`, and `public-read-write` policies with bucket/object-level evaluation during requests.

### 2. Server-Side Encryption (SSE) & Presigned URLs
- **Encryption in Transit / At Rest**: Implement AES-256-GCM encryption streams seamlessly at the `PutObject` / `GetObject` layer, caching cryptographic nonces securely in `bbolt`.
- **Presigned URLs**: Enhance the current AWS SigV4 implementation to support `X-Amz-Signature` query parameters and `X-Amz-Expires` for temporary, time-bound, secure download and upload URLs without needing the Authorization header.

### 3. Lifecycle Engine & Object Tagging
- **Lifecycle Expiration**: Introduce background worker goroutines to evaluate rules that automatically expire and purge objects based on configured TTLs (Time-To-Live).
- **Object Tagging**: Support attaching up to 10 Key/Value tags per object in metadata (`PutObjectTagging`, `GetObjectTagging`).
- **CORS Support**: Enforce Cross-Origin Resource Sharing (CORS) rules during browser Preflight `OPTIONS` requests, permitting direct web browser uploads/downloads to `sssstore` from specific domains.

### 4. Rate Limiting Limits & Audit Trail
- **Token-Bucket Routing**: Prevent local or distributed Denial-of-Service by capping Requests Per Second (RPS) per AccessKey or IP via a robust limiter middleware.
- **Enhanced Auditing**: Evolve the existing audit framework into a structured, queryable JSON stream that can optionally sync to `syslog` or an ELK stack aggregator.

---

## Phase 3: Scale & Enterprise Features

### 1. S3 Event Webhooks & Storage Quotas
- **Quotas**: Support and enforce configurable byte limits either per-bucket or per-user.
- **Event Notifications**: Dispatch HTTP webhooks to external servers exactly mirroring S3 event payloads (e.g., `ObjectCreated:Put`) upon modifications.

### 2. Static Website Hosting
- **Subdomain Routing**: Implement virtual-host style routing (`<bucket>.example.com`) overriding S3 endpoint requests to directly serve an internal `index.html` directory listing and `error.html` for 4xx conditions.

### 3. Asynchronous Object Replication
- **Replication Targets**: Introduce async dispatchers capable of mirroring newly uploaded objects to secondary `sssstore` clusters for fault tolerance, or directly backing them up to AWS S3, GCS, or Azure Blob Storage.

### 4. External Identity Brokers (SSO/OIDC)
- **OIDC/LDAP Single Sign-On**: Allow human/admin users to interact via a web portal using Keycloak, Okta, or GitHub. Trade the JSON Web Token (JWT) securely for short-lived IAM-restricted S3 session keys.

### 5. Multi-Tier Storage Policies
- **Cold Storage Routing**: Extend the internal `StorageEngine` and `bbolt` metadata to categorize Hot, Warm, and Cold objects, automatically shifting non-frequently accessed binaries transparently to remote backend proxies like S3 Glacier after X days to alleviate expensive local disk stress.
