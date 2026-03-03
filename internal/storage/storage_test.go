package storage

import (
	"io"
	"strings"
	"testing"
)

func TestBucketAndObjectLifecycle(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)

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
