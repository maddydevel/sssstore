# User Guide

## Authentication model

All S3 endpoints require an `Authorization` header with SigV4-like credential scope. The server validates the `Credential=<access-key>/...` value.

## Buckets

Supported bucket-level actions:
- Create bucket
- Delete bucket
- Head bucket
- List buckets
- Get/put versioning state
- List object versions

## Objects

Supported object actions:
- Put/Get/Head/Delete object
- Range GET
- List objects (V2)
- Version-aware read/delete using `versionId`

## Multipart uploads

Supported multipart flow:
1. initiate `POST ?uploads`
2. upload parts `PUT ?partNumber=&uploadId=`
3. complete `POST ?uploadId=`
4. abort `DELETE ?uploadId=`

## Tips

- Prefer unique bucket naming conventions by environment (`dev-`, `stg-`, `prod-`).
- Enable versioning for critical data retention workflows.
- Use lifecycle and scrub tools periodically in long-running installations.
