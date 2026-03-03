package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrBucketNotEmpty = errors.New("bucket not empty")
	ErrObjectNotFound = errors.New("object not found")
)

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}

type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: filepath.Join(root, "buckets")}
}

func (s *Store) bucketDir(bucket string) string { return filepath.Join(s.root, bucket) }
func (s *Store) objectsDir(bucket string) string {
	return filepath.Join(s.bucketDir(bucket), "objects")
}

func sanitizeSegment(seg string) bool {
	return seg != "" && seg != "." && seg != ".." && !strings.Contains(seg, "\\")
}

func validateBucket(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}
	for _, ch := range bucket {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func cleanKey(key string) (string, bool) {
	parts := strings.Split(key, "/")
	for _, p := range parts {
		if !sanitizeSegment(p) {
			return "", false
		}
	}
	return filepath.Join(parts...), true
}

func (s *Store) CreateBucket(bucket string) error {
	if !validateBucket(bucket) {
		return errors.New("invalid bucket name")
	}
	return os.MkdirAll(s.objectsDir(bucket), 0o755)
}

func (s *Store) DeleteBucket(bucket string) error {
	objDir := s.objectsDir(bucket)
	if _, err := os.Stat(objDir); err != nil {
		if os.IsNotExist(err) {
			return ErrBucketNotFound
		}
		return err
	}
	hasFiles := false
	err := filepath.Walk(objDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			hasFiles = true
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if hasFiles {
		return ErrBucketNotEmpty
	}
	return os.RemoveAll(s.bucketDir(bucket))
}

func (s *Store) ListBuckets() ([]BucketInfo, error) {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]BucketInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		st, err := os.Stat(filepath.Join(s.root, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, BucketInfo{Name: e.Name(), CreationDate: st.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) PutObject(bucket, key string, r io.Reader) (ObjectInfo, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return ObjectInfo{}, errors.New("invalid key")
	}
	if _, err := os.Stat(s.objectsDir(bucket)); err != nil {
		if os.IsNotExist(err) {
			return ObjectInfo{}, ErrBucketNotFound
		}
		return ObjectInfo{}, err
	}
	p := filepath.Join(s.objectsDir(bucket), clean)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return ObjectInfo{}, err
	}
	f, err := os.Create(p)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return ObjectInfo{}, err
	}
	st, _ := f.Stat()
	return ObjectInfo{Key: key, Size: n, LastModified: st.ModTime().UTC(), ETag: ""}, nil
}

func (s *Store) GetObject(bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	p := filepath.Join(s.objectsDir(bucket), clean)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ObjectInfo{}, ErrObjectNotFound
		}
		return nil, ObjectInfo{}, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, ObjectInfo{}, err
	}
	return f, ObjectInfo{Key: key, Size: st.Size(), LastModified: st.ModTime().UTC(), ETag: ""}, nil
}

func (s *Store) DeleteObject(bucket, key string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	p := filepath.Join(s.objectsDir(bucket), clean)
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return err
	}
	return nil
}

func (s *Store) ListObjectsV2(bucket, prefix string, maxKeys int) ([]ObjectInfo, error) {
	root := s.objectsDir(bucket)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBucketNotFound
		}
		return nil, err
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	objs := make([]ObjectInfo, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		objs = append(objs, ObjectInfo{Key: key, Size: info.Size(), LastModified: info.ModTime().UTC()})
		if len(objs) >= maxKeys {
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Key < objs[j].Key })
	return objs, nil
}
