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
python3 - <<PY
import json
p = r"$CONFIG_PATH"
with open(p, "r", encoding="utf-8") as f:
    cfg = json.load(f)
cfg["bind_addr"] = ":$PORT"
with open(p, "w", encoding="utf-8") as f:
    json.dump(cfg, f, indent=2)
PY
"$BIN" user create --config "$CONFIG_PATH" --name qa --access-key qa-key --secret-key qa-secret --policy admin >/dev/null

echo "[qa] start server"
"$BIN" server --config "$CONFIG_PATH" >/tmp/sssstore-qa-server.log 2>&1 &
SPID=$!
sleep 1

echo "[qa] creating signing utility"
cat > "${TMPDIR_QA}/sign.py" << 'EOF'
import sys, hmac, hashlib, datetime, urllib.parse

def url_encode(s, path=False):
    res = []
    for c in s.encode('utf-8'):
        if (c >= ord('a') and c <= ord('z')) or (c >= ord('A') and c <= ord('Z')) or (c >= ord('0') and c <= ord('9')) or chr(c) in '-_.~':
            res.append(chr(c))
        elif path and chr(c) == '/':
            res.append(chr(c))
        elif not path and chr(c) == ' ':
            res.append("%20")
        else:
            res.append(f"%{c:02X}")
    return "".join(res)

def sign(method, url, access_key, secret_key, region='us-east-1', service='s3'):
    t = datetime.datetime.now(datetime.UTC)
    amz_date = t.strftime('%Y%m%dT%H%M%SZ')
    date_stamp = t.strftime('%Y%m%d')
    parsed = urllib.parse.urlparse(url)
    
    query_parts = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
    canonical_query = "&".join(f"{url_encode(k)}={url_encode(v)}" for k, v in sorted(query_parts))
    if canonical_query == "" and "=" in parsed.query:
        # handle ?uploads which parse_qsl ignores if keep_blank_values=False but we set it true
        pass
    if parsed.query and not "=" in parsed.query:
        canonical_query = f"{url_encode(parsed.query)}="
    elif parsed.query:
        # Need to handle a=b&c edge case
        qp2 = []
        for p in parsed.query.split("&"):
            if "=" in p:
                k,v = p.split("=", 1)
                qp2.append((url_encode(k), url_encode(v)))
            else:
                qp2.append((url_encode(p), ""))
        qp2.sort()
        canonical_query = "&".join(f"{k}={v}" for k, v in qp2)

    payload_hash = "UNSIGNED-PAYLOAD"
    canonical_headers = f"host:{parsed.netloc}\nx-amz-date:{amz_date}\n"
    signed_headers = "host;x-amz-date"
    
    canonical_uri = url_encode(parsed.path or '/', path=True)
    canonical_request = f"{method}\n{canonical_uri}\n{canonical_query}\n{canonical_headers}\n{signed_headers}\n{payload_hash}"
    algorithm = "AWS4-HMAC-SHA256"
    credential_scope = f"{date_stamp}/{region}/{service}/aws4_request"
    string_to_sign = f"{algorithm}\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode('utf-8')).hexdigest()}"
    
    def sign_msg(key, msg): return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()
    
    k_date = sign_msg(('AWS4' + secret_key).encode('utf-8'), date_stamp)
    k_region = sign_msg(k_date, region)
    k_service = sign_msg(k_region, service)
    k_signing = sign_msg(k_service, "aws4_request")
    
    signature = hmac.new(k_signing, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()
    auth_header = f"{algorithm} Credential={access_key}/{credential_scope}, SignedHeaders={signed_headers}, Signature={signature}"
    return auth_header, amz_date

if __name__ == '__main__':
    a, d = sign(sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4])
    print(f"-H 'Authorization: {a}' -H 'x-amz-date: {d}'")
EOF

run_curl() {
    local method="$1"
    local url="$2"
    shift 2
    local auth_args
    auth_args=$(python3 "${TMPDIR_QA}/sign.py" "$method" "$url" "qa-key" "qa-secret")
    eval "curl -sS $auth_args -X $method \"\$url\" \"\$@\""
}

echo "[qa] system checks"
curl -sSf "${BASE_URL}/healthz" >/dev/null
curl -sSf "${BASE_URL}/readyz" >/dev/null
curl -sSf "${BASE_URL}/metrics" >/dev/null

run_curl PUT "${BASE_URL}/sysbucket" -o /dev/null -w '%{http_code}' | grep -q '^200$'
run_curl PUT "${BASE_URL}/sysbucket/obj.txt" --data-binary "system-test" -o /dev/null -w '%{http_code}' | grep -q '^200$'
[[ "$(run_curl GET "${BASE_URL}/sysbucket/obj.txt")" == "system-test" ]]

echo "[qa] acceptance checks"
run_curl PUT "${BASE_URL}/accbucket" -o /dev/null -w '%{http_code}' | grep -q '^200$'
UPLOAD_XML="$(run_curl POST "${BASE_URL}/accbucket/video.bin?uploads")"
UPLOAD_ID="$(echo "$UPLOAD_XML" | sed -n 's:.*<UploadId>\(.*\)</UploadId>.*:\1:p')"
[[ -n "$UPLOAD_ID" ]]
run_curl PUT "${BASE_URL}/accbucket/video.bin?partNumber=1&uploadId=${UPLOAD_ID}" --data-binary "hello " -o /dev/null -w '%{http_code}' | grep -q '^200$'
run_curl PUT "${BASE_URL}/accbucket/video.bin?partNumber=2&uploadId=${UPLOAD_ID}" --data-binary "acceptance" -o /dev/null -w '%{http_code}' | grep -q '^200$'
run_curl POST "${BASE_URL}/accbucket/video.bin?uploadId=${UPLOAD_ID}" -o /dev/null -w '%{http_code}' | grep -q '^200$'
[[ "$(run_curl GET "${BASE_URL}/accbucket/video.bin")" == "hello acceptance" ]]

echo "[qa] security checks"
go test -run TestLoadStrictModeValidation ./internal/config >/dev/null
curl -sS -o /tmp/qa-denied.xml -w '%{http_code}' "${BASE_URL}/" | grep -q '^403$'

echo "[qa] performance smoke"
python3 - <<PY
import time, urllib.request, sys
sys.path.append(r"${TMPDIR_QA}")
import sign
N=30
url=r"$BASE_URL/sysbucket/obj.txt"
auth_header, amz_date = sign.sign("GET", url, "qa-key", "qa-secret")
headers={"Authorization": auth_header, "x-amz-date": amz_date}
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
