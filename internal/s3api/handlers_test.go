package s3api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sssstore/sssstore/internal/storage"
)

func TestS3BasicFlow(t *testing.T) {
	tmp := t.TempDir()
	h := New(storage.New(tmp))
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/mybucket", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create bucket status: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodHead, ts.URL+"/mybucket", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("head bucket status: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/mybucket/a.txt", strings.NewReader("abcdef"))
	resp, err = http.DefaultClient.Do(req)
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

	resp, err = http.Get(ts.URL + "/mybucket/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "abcdef" {
		t.Fatalf("expected abcdef got %s", string(b))
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/mybucket/a.txt", nil)
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

	resp, err = http.Get(ts.URL + "/mybucket?list-type=2&max-keys=1")
	if err != nil {
		t.Fatal(err)
	}
	lb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(lb), "<Key>a.txt</Key>") {
		t.Fatalf("list output missing key: %s", string(lb))
	}
}
