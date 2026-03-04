# API Guide

This document summarizes currently supported S3-style HTTP endpoints.

## Authentication

All S3 endpoints require:
- `Authorization: AWS4-HMAC-SHA256 Credential=<access-key>/...`

## Bucket APIs

- `GET /` → ListBuckets
- `PUT /{bucket}` → CreateBucket
- `HEAD /{bucket}` → HeadBucket
- `DELETE /{bucket}` → DeleteBucket
- `GET /{bucket}?list-type=2` → ListObjectsV2
- `PUT /{bucket}?versioning` → PutBucketVersioning
- `GET /{bucket}?versioning` → GetBucketVersioning
- `GET /{bucket}?versions` → ListObjectVersions

## Object APIs

- `PUT /{bucket}/{key}` → PutObject
- `GET /{bucket}/{key}` → GetObject
- `HEAD /{bucket}/{key}` → HeadObject
- `DELETE /{bucket}/{key}` → DeleteObject

Version-aware variants:
- `GET /{bucket}/{key}?versionId=...`
- `HEAD /{bucket}/{key}?versionId=...`
- `DELETE /{bucket}/{key}?versionId=...`

Range support:
- `GET /{bucket}/{key}` with `Range: bytes=start-end`

## Multipart APIs

- `POST /{bucket}/{key}?uploads` → InitiateMultipartUpload
- `PUT /{bucket}/{key}?partNumber=N&uploadId=...` → UploadPart
- `POST /{bucket}/{key}?uploadId=...` → CompleteMultipartUpload
- `DELETE /{bucket}/{key}?uploadId=...` → AbortMultipartUpload

## Response format

- XML is used for S3-style API responses and errors.
- Typical S3-compatible status codes are returned where implemented.
