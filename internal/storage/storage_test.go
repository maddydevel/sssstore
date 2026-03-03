package storage

import (
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

	objs, err := s.ListObjectsV2("test-bucket", "folder/", 1000)
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 1 || objs[0].Key != "folder/hello.txt" {
		t.Fatalf("unexpected objects: %+v", objs)
	}

	if err := s.DeleteObject("test-bucket", "folder/hello.txt"); err != nil {
		t.Fatalf("delete object: %v", err)
	}
	if err := s.DeleteBucket("test-bucket"); err != nil {
		t.Fatalf("delete bucket: %v", err)
	}
}
