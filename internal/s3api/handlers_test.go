package s3api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sssstore/sssstore/internal/auth"
	"github.com/sssstore/sssstore/internal/storage"
)

func sigV4Header(key string) string {
	return "AWS4-HMAC-SHA256 Credential=" + key + "/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc"
}

type mockAuth struct {
	allowed map[string]string
}
func (m *mockAuth) Authenticate(r *http.Request) (auth.Principal, error) {
	key := auth.AccessKeyFromRequest(r)
	if p, ok := m.allowed[key]; ok {
		return auth.Principal{AccessKey: key, Policy: p}, nil
	}
	return auth.Principal{}, auth.ErrAccessDenied
}
func newMockAuth(keys []string) *mockAuth {
	m := make(map[string]string)
	for _, k := range keys {
		m[k] = auth.PolicyAdmin
	}
	return &mockAuth{allowed: m}
}


func authedReq(method, url, body, key string) *http.Request {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Authorization", sigV4Header(key))
	return req
}

func TestS3BasicFlow(t *testing.T) {
	tmp := t.TempDir()
	h := New(storage.New(tmp), newMockAuth([]string{"testkey"}))
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/mybucket", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create bucket status: %d", resp.StatusCode)
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodHead, ts.URL+"/mybucket", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("head bucket status: %d", resp.StatusCode)
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/mybucket/a.txt", "abcdef", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put object status: %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatal("expected etag header")
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "abcdef" {
		t.Fatalf("expected abcdef got %s", string(b))
	}

	req := authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey")
	req.Header.Set("Range", "bytes=1-3")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || string(rb) != "bcd" {
		t.Fatalf("unexpected range response status=%d body=%s", resp.StatusCode, string(rb))
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodGet, ts.URL+"/mybucket?list-type=2&max-keys=1", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	lb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(lb), "<Key>a.txt</Key>") {
		t.Fatalf("list output missing key: %s", string(lb))
	}

	// Test Conditionals
	req = authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey")
	req.Header.Set("If-Match", `"wrong-etag"`)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("If-Match wrong etag expected 412, got %d", resp.StatusCode)
	}

	req = authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey")
	req.Header.Set("If-None-Match", `*`)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match * expected 304, got %d", resp.StatusCode)
	}

	req = authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey")
	req.Header.Set("If-Unmodified-Since", "Thu, 01 Jan 1970 00:00:00 GMT")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("If-Unmodified-Since past expected 412, got %d", resp.StatusCode)
	}

	req = authedReq(http.MethodGet, ts.URL+"/mybucket/a.txt", "", "testkey")
	req.Header.Set("If-Modified-Since", "Thu, 01 Jan 2099 00:00:00 GMT")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("If-Modified-Since future expected 304, got %d", resp.StatusCode)
	}
}

func TestAuthDenied(t *testing.T) {
	tmp := t.TempDir()
	h := New(storage.New(tmp), newMockAuth([]string{"allowed"}))
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden got %d", resp.StatusCode)
	}
}

func TestMultipartFlow(t *testing.T) {
	tmp := t.TempDir()
	h := New(storage.New(tmp), newMockAuth([]string{"testkey"}))
	ts := httptest.NewServer(h)
	defer ts.Close()

	_, _ = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/mpbucket", "", "testkey"))

	resp, err := http.DefaultClient.Do(authedReq(http.MethodPost, ts.URL+"/mpbucket/obj?uploads", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("init multipart: %d %s", resp.StatusCode, string(body))
	}
	xml := string(body)
	start := strings.Index(xml, "<UploadId>")
	end := strings.Index(xml, "</UploadId>")
	if start < 0 || end < 0 {
		t.Fatalf("upload id missing: %s", xml)
	}
	uploadID := xml[start+10 : end]

	resp, err = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/mpbucket/obj?partNumber=1&uploadId="+uploadID, "hello ", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload part1: %d", resp.StatusCode)
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/mpbucket/obj?partNumber=2&uploadId="+uploadID, "world", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload part2: %d", resp.StatusCode)
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodPost, ts.URL+"/mpbucket/obj?uploadId="+uploadID, "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete multipart: %d", resp.StatusCode)
	}

	resp, err = http.DefaultClient.Do(authedReq(http.MethodGet, ts.URL+"/mpbucket/obj", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b2) != "hello world" {
		t.Fatalf("expected merged object, got %q", string(b2))
	}
}

func TestVersioningFlow(t *testing.T) {
	tmp := t.TempDir()
	h := New(storage.New(tmp), newMockAuth([]string{"testkey"}))
	ts := httptest.NewServer(h)
	defer ts.Close()

	_, _ = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/verbucket", "", "testkey"))
	vxml := `<VersioningConfiguration><Status>Enabled</Status></VersioningConfiguration>`
	resp, err := http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/verbucket?versioning", vxml, "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable versioning status=%d", resp.StatusCode)
	}

	_, _ = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/verbucket/file.txt", "one", "testkey"))
	_, _ = http.DefaultClient.Do(authedReq(http.MethodPut, ts.URL+"/verbucket/file.txt", "two", "testkey"))

	resp, err = http.DefaultClient.Do(authedReq(http.MethodGet, ts.URL+"/verbucket?versions", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "<VersionID>") {
		t.Fatalf("expected versions listing, got %s", string(b))
	}

	_, _ = http.DefaultClient.Do(authedReq(http.MethodDelete, ts.URL+"/verbucket/file.txt", "", "testkey"))
	resp, err = http.DefaultClient.Do(authedReq(http.MethodGet, ts.URL+"/verbucket/file.txt", "", "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected not found after delete marker, got %d", resp.StatusCode)
	}
}
