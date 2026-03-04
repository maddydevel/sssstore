package storage

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestBucketAndObjectLifecycle(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	defer s.Close()

	if err := s.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	_, err := s.PutObject("test-bucket", "folder/hello.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("put object: %v", err)
	}
	rc, info, err := s.GetObject("test-bucket", "folder/hello.txt")
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	_ = rc.Close()
	if info.Size != 5 {
		t.Fatalf("expected size 5 got %d", info.Size)
	}
	if info.ETag == "" {
		t.Fatal("expected etag")
	}

	objs, next, truncated, err := s.ListObjectsV2("test-bucket", "folder/", "", 1000)
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 1 || objs[0].Key != "folder/hello.txt" || next != "" || truncated {
		t.Fatalf("unexpected objects: %+v next=%s truncated=%v", objs, next, truncated)
	}

	if err := s.DeleteObject("test-bucket", "folder/hello.txt"); err != nil {
		t.Fatalf("delete object: %v", err)
	}
	if err := s.DeleteBucket("test-bucket"); err != nil {
		t.Fatalf("delete bucket: %v", err)
	}
}

func TestMultipartLifecycle(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	defer s.Close()
	if err := s.CreateBucket("mp-bucket"); err != nil {
		t.Fatal(err)
	}
	uploadID, err := s.CreateMultipartUpload("mp-bucket", "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UploadPart("mp-bucket", "big.bin", uploadID, 1, strings.NewReader("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UploadPart("mp-bucket", "big.bin", uploadID, 2, strings.NewReader("def")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteMultipartUpload("mp-bucket", "big.bin", uploadID); err != nil {
		t.Fatal(err)
	}
	rc, _, err := s.GetObject("mp-bucket", "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "abcdef" {
		t.Fatalf("unexpected object %q", string(b))
	}
}

func TestCleanupStaleMultipartUploads(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	defer s.Close()
	if err := s.CreateBucket("clean-bucket"); err != nil {
		t.Fatal(err)
	}
	uploadID, err := s.CreateMultipartUpload("clean-bucket", "old.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UploadPart("clean-bucket", "old.bin", uploadID, 1, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	removed, err := s.CleanupStaleMultipartUploads(0)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected zero removed, got %d", removed)
	}
	removed, err = s.CleanupStaleMultipartUploads(-1 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("expected zero removed for invalid maxAge, got %d", removed)
	}
	removed, err = s.CleanupStaleMultipartUploads(1 * time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	if removed < 1 {
		t.Fatalf("expected stale uploads removal, got %d", removed)
	}
}

func TestVersioningLifecycle(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	defer s.Close()
	if err := s.CreateBucket("v-bucket"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBucketVersioning("v-bucket", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutObject("v-bucket", "k.txt", strings.NewReader("one")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutObject("v-bucket", "k.txt", strings.NewReader("two")); err != nil {
		t.Fatal(err)
	}
	versions, err := s.ListObjectVersions("v-bucket", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) < 2 {
		t.Fatalf("expected at least 2 versions, got %d", len(versions))
	}
	if err := s.DeleteObject("v-bucket", "k.txt"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetObject("v-bucket", "k.txt"); err == nil {
		t.Fatal("expected not found due to delete marker")
	}
}

func TestScrubRepairRebuildsMissingMeta(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	if err := s.CreateBucket("scrub-bucket"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutObject("scrub-bucket", "a.txt", strings.NewReader("alpha")); err != nil {
		t.Fatal(err)
	}

	// Manually delete the meta from the bolt DB
	if err := s.meta.DeleteObjectMeta("scrub-bucket", "a.txt"); err != nil {
		t.Fatal(err)
	}

	report, err := s.Scrub(true)
	if err != nil {
		t.Fatal(err)
	}
	if report.MissingMeta < 1 {
		t.Fatalf("expected missing meta detection, got %+v", report)
	}

	// Verify etag rebuilt
	_, info, err := s.GetObject("scrub-bucket", "a.txt")
	if err != nil {
		t.Fatalf("get object failed: %v", err)
	}
	if info.ETag == "" {
		t.Fatal("expected etag to be rebuilt")
	}
	_ = s.Close()
}

func TestScrubRepairRemovesOrphanMeta(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	if err := s.CreateBucket("orphan-bucket"); err != nil {
		t.Fatal(err)
	}

	// Insert orphan metadata into bolt DB
	if err := s.meta.PutObjectMeta("orphan-bucket", "ghost.txt", "\"x\""); err != nil {
		t.Fatal(err)
	}

	report, err := s.Scrub(true)
	if err != nil {
		t.Fatal(err)
	}
	if report.OrphanMeta < 1 {
		t.Fatalf("expected orphan meta detection, got %+v", report)
	}

	if _, err := s.meta.GetObjectMeta("orphan-bucket", "ghost.txt"); err == nil {
		t.Fatal("expected orphan meta removed")
	}
	_ = s.Close()
}
