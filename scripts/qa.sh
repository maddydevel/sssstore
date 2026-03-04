#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN="/tmp/sssstore-qa"
TMPDIR_QA="$(mktemp -d)"
PORT="19000"
BASE_URL="http://127.0.0.1:${PORT}"
CONFIG_PATH="${TMPDIR_QA}/sssstore.json"
DATA_DIR="${TMPDIR_QA}/data"

cleanup() {
  if [[ -n "${SPID:-}" ]]; then
    kill "$SPID" >/dev/null 2>&1 || true
    wait "$SPID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMPDIR_QA" "$BIN" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[qa] building binary"
go build -o "$BIN" ./cmd/sssstore

echo "[qa] unit tests"
go test ./internal/config ./internal/storage ./internal/s3api

echo "[qa] integration tests"
go test -run 'TestS3BasicFlow|TestMultipartFlow|TestVersioningFlow|TestAuthDenied' ./internal/s3api
go test -run 'TestBucketAndObjectLifecycle|TestMultipartLifecycle|TestVersioningLifecycle|TestScrubRepairRebuildsMissingMeta|TestScrubRepairRemovesOrphanMeta|TestCleanupStaleMultipartUploads' ./internal/storage

echo "[qa] setup runtime"
"$BIN" init --config "$CONFIG_PATH" --data "$DATA_DIR" >/dev/null
python - <<PY
import json
p = r"$CONFIG_PATH"
with open(p, "r", encoding="utf-8") as f:
    cfg = json.load(f)
cfg["bind_addr"] = ":$PORT"
with open(p, "w", encoding="utf-8") as f:
    json.dump(cfg, f, indent=2)
PY
"$BIN" user create --config "$CONFIG_PATH" --name qa --access-key qa-key --secret-key qa-secret >/dev/null

echo "[qa] start server"
"$BIN" server --config "$CONFIG_PATH" >/tmp/sssstore-qa-server.log 2>&1 &
SPID=$!
sleep 1

AUTH='Authorization: AWS4-HMAC-SHA256 Credential=qa-key/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy'

echo "[qa] system checks"
curl -sSf "${BASE_URL}/healthz" >/dev/null
curl -sSf "${BASE_URL}/readyz" >/dev/null
curl -sSf "${BASE_URL}/metrics" >/dev/null
curl -sS -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/sysbucket" -H "$AUTH" | grep -q '^200$'
curl -sS -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/sysbucket/obj.txt" -H "$AUTH" --data-binary 'system-test' | grep -q '^200$'
[[ "$(curl -s -H "$AUTH" "${BASE_URL}/sysbucket/obj.txt")" == "system-test" ]]

echo "[qa] acceptance checks"
curl -sS -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/accbucket" -H "$AUTH" | grep -q '^200$'
UPLOAD_XML="$(curl -sS -X POST "${BASE_URL}/accbucket/video.bin?uploads" -H "$AUTH")"
UPLOAD_ID="$(echo "$UPLOAD_XML" | sed -n 's:.*<UploadId>\(.*\)</UploadId>.*:\1:p')"
[[ -n "$UPLOAD_ID" ]]
curl -sS -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/accbucket/video.bin?partNumber=1&uploadId=${UPLOAD_ID}" -H "$AUTH" --data-binary 'hello ' | grep -q '^200$'
curl -sS -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/accbucket/video.bin?partNumber=2&uploadId=${UPLOAD_ID}" -H "$AUTH" --data-binary 'acceptance' | grep -q '^200$'
curl -sS -o /dev/null -w '%{http_code}' -X POST "${BASE_URL}/accbucket/video.bin?uploadId=${UPLOAD_ID}" -H "$AUTH" | grep -q '^200$'
[[ "$(curl -s -H "$AUTH" "${BASE_URL}/accbucket/video.bin")" == "hello acceptance" ]]

echo "[qa] security checks"
go test -run TestLoadStrictModeValidation ./internal/config >/dev/null
curl -sS -o /tmp/qa-denied.xml -w '%{http_code}' "${BASE_URL}/" | grep -q '^403$'

echo "[qa] performance smoke"
python - <<PY
import time, urllib.request
N=30
url=r"$BASE_URL/sysbucket/obj.txt"
headers={"Authorization":"AWS4-HMAC-SHA256 Credential=qa-key/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy"}
lat=[]
for _ in range(N):
    req=urllib.request.Request(url, headers=headers)
    t=time.perf_counter()
    with urllib.request.urlopen(req, timeout=5) as r:
        r.read()
    lat.append((time.perf_counter()-t)*1000)
lat.sort()
print(f"[qa] perf: requests={N} avg_ms={sum(lat)/N:.2f} p95_ms={lat[int(N*0.95)-1]:.2f}")
PY

echo "[qa] usability smoke"
set +e
"$BIN" >/tmp/qa-usage.txt 2>&1
EC=$?
set -e
[[ "$EC" == "2" ]]
grep -q "sssstore commands" /tmp/qa-usage.txt

"$BIN" doctor --config "$CONFIG_PATH" --scrub >/dev/null

echo "[qa] all checks passed"
