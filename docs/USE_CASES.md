# Best Use Cases

## 1) Local S3-compatible development

Use sssstore to run integration tests against S3-like APIs without relying on public cloud dependencies.

## 2) CI/CD artifacts and backups

Store build artifacts, release bundles, and backup objects in a privately managed store.

## 3) Edge or on-prem object storage

Deploy in constrained or isolated environments where managed cloud object storage is unavailable.

## 4) Internal app static/object assets

Use bucket/object APIs to host and serve internal binary files, documents, and media objects.

## 5) Learning and prototyping S3 workflows

Test multipart uploads, versioning behavior, and operational workflows in a small footprint environment.

## When not to use (yet)

- Large, globally distributed multi-region requirements
- Full AWS S3 edge-case parity expectations
- Advanced IAM/STS policy ecosystems requiring complete compatibility
