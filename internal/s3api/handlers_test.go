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

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/mybucket/a.txt", strings.NewReader("abc"))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put object status: %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/mybucket/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "abc" {
		t.Fatalf("expected abc got %s", string(b))
	}

	resp, err = http.Get(ts.URL + "/mybucket?list-type=2")
	if err != nil {
		t.Fatal(err)
	}
	lb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(lb), "<Key>a.txt</Key>") {
		t.Fatalf("list output missing key: %s", string(lb))
	}
}
