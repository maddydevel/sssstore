# S3 Compatibility Matrix (Current)

## Supported

- ListBuckets
- CreateBucket / DeleteBucket / HeadBucket
- PutObject / GetObject / HeadObject / DeleteObject
- ListObjectsV2
- Multipart: create/upload-part/complete/abort
- Bucket versioning get/put and list versions

## Partial / Notes

- SigV4 authentication is scaffolded by validating access key from credential scope.
- Not all AWS edge-case semantics are implemented.

## Not yet supported

- ACL APIs
- STS temporary credentials
- Cross-region replication orchestration
- Full IAM condition evaluation semantics
