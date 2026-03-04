# Examples

## Initialize and run

```bash
sssstore init --config ./sssstore.json --data ./data
sssstore server --config ./sssstore.json
```

## Create a user

```bash
sssstore user create --config ./sssstore.json --name app --access-key app-key --secret-key app-secret
```

## Create a bucket (raw HTTP)

```bash
curl -X PUT http://localhost:9000/my-bucket -H 'Authorization: AWS4-HMAC-SHA256 Credential=app-key/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy'
```

## Upload an object

```bash
curl -X PUT http://localhost:9000/my-bucket/file.txt \
  -H 'Authorization: AWS4-HMAC-SHA256 Credential=app-key/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy' \
  --data-binary 'hello world'
```

## Enable versioning

```bash
curl -X PUT 'http://localhost:9000/my-bucket?versioning' \
  -H 'Authorization: AWS4-HMAC-SHA256 Credential=app-key/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy' \
  -H 'Content-Type: application/xml' \
  --data '<VersioningConfiguration><Status>Enabled</Status></VersioningConfiguration>'
```

## Doctor scrub

```bash
sssstore doctor --config ./sssstore.json --scrub
sssstore doctor --config ./sssstore.json --scrub --repair
```
