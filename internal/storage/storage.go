package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
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

// MetadataStore abstracts bucket/object metadata concerns.
type MetadataStore interface {
	ListBuckets() ([]BucketInfo, error)
	BucketExists(bucket string) (bool, error)
	CreateBucket(bucket string) error
	DeleteBucket(bucket string) error
	PutObjectMeta(bucket, key, etag string) error
	GetObjectMeta(bucket, key string) (etag string, _ error)
	DeleteObjectMeta(bucket, key string) error
}

// ObjectBackend abstracts object data persistence.
type ObjectBackend interface {
	PutObject(bucket, key string, r io.Reader) (ObjectInfo, error)
	GetObject(bucket, key string) (io.ReadCloser, ObjectInfo, error)
	DeleteObject(bucket, key string) error
	ListObjects(bucket, prefix, continuationToken string, maxKeys int) ([]ObjectInfo, string, bool, error)
}

type Store struct {
	meta           MetadataStore
	objects        ObjectBackend
	replicationDir string
	db             *bbolt.DB
}

func New(root string) *Store {
	base := filepath.Join(root, "buckets")
	_ = os.MkdirAll(base, 0o755)

	dbPath := filepath.Join(base, "metadata.db")
	dbExists := false
	if _, err := os.Stat(dbPath); err == nil {
		dbExists = true
	}

	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		panic("failed to open bbolt metadata db: " + err.Error())
	}

	boltMeta, err := newBboltMetadataStore(db, base)
	if err != nil {
		panic("failed to init bbolt metadata store: " + err.Error())
	}

	if !dbExists {
		// Run a one-time migration from .meta.json to bbolt
		migrateFSMetadataToBbolt(base, boltMeta)
	}

	return &Store{
		meta:    boltMeta,
		objects: newFSObjectBackend(base),
		db:      db,
	}
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func migrateFSMetadataToBbolt(root string, meta MetadataStore) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".meta.json") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		parts := strings.SplitN(rel, string(filepath.Separator), 3)
		if len(parts) < 3 || parts[1] != "objects" {
			return nil
		}

		bucket := parts[0]
		keyMeta := filepath.ToSlash(parts[2])
		key := strings.TrimSuffix(keyMeta, ".meta.json")

		b, err := os.ReadFile(path)
		if err == nil {
			var om objectMeta
			if json.Unmarshal(b, &om) == nil {
				_ = meta.CreateBucket(bucket)
				_ = meta.PutObjectMeta(bucket, key, om.ETag)
			}
		}

		// Remove migrated legacy file
		_ = os.Remove(path)
		return nil
	})
}

func (s *Store) EnableReplication(dir string) {
	s.replicationDir = dir
}

func NewWithBackends(meta MetadataStore, objects ObjectBackend) *Store {
	return &Store{meta: meta, objects: objects}
}

func (s *Store) BucketExists(bucket string) (bool, error) { return s.meta.BucketExists(bucket) }
func (s *Store) CreateBucket(bucket string) error         { return s.meta.CreateBucket(bucket) }
func (s *Store) DeleteBucket(bucket string) error         { return s.meta.DeleteBucket(bucket) }
func (s *Store) ListBuckets() ([]BucketInfo, error)       { return s.meta.ListBuckets() }

func (s *Store) PutObject(bucket, key string, r io.Reader) (ObjectInfo, error) {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil && !errors.Is(err, ErrBucketNotFound) {
		return ObjectInfo{}, err
	}
	if enabled {
		info, _, err := s.putObjectVersioned(bucket, key, r)
		if err == nil {
			s.replicatePut(bucket, key)
		}
		return info, err
	}
	info, err := s.objects.PutObject(bucket, key, r)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := s.meta.PutObjectMeta(bucket, key, info.ETag); err != nil {
		return ObjectInfo{}, err
	}
	s.replicatePut(bucket, key)
	return info, nil
}

func (s *Store) GetObject(bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil && !errors.Is(err, ErrBucketNotFound) {
		return nil, ObjectInfo{}, err
	}
	if enabled {
		return s.GetObjectVersion(bucket, key, "")
	}
	rc, info, err := s.objects.GetObject(bucket, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	etag, err := s.meta.GetObjectMeta(bucket, key)
	if err == nil {
		info.ETag = etag
	}
	return rc, info, nil
}

func (s *Store) DeleteObject(bucket, key string) error {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil && !errors.Is(err, ErrBucketNotFound) {
		return err
	}
	if enabled {
		return s.DeleteObjectVersion(bucket, key, "")
	}
	if err := s.objects.DeleteObject(bucket, key); err != nil {
		return err
	}
	_ = s.meta.DeleteObjectMeta(bucket, key)
	s.replicateDelete(bucket, key)
	return nil
}

func (s *Store) ListObjectsV2(bucket, prefix, continuationToken string, maxKeys int) ([]ObjectInfo, string, bool, error) {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil && !errors.Is(err, ErrBucketNotFound) {
		return nil, "", false, err
	}
	if enabled {
		vers, err := s.ListObjectVersions(bucket, prefix, maxKeys)
		if err != nil {
			return nil, "", false, err
		}
		objs := make([]ObjectInfo, 0, len(vers))
		seen := map[string]struct{}{}
		for _, v := range vers {
			if !v.IsLatest || v.DeleteMarker {
				continue
			}
			if _, ok := seen[v.Key]; ok {
				continue
			}
			seen[v.Key] = struct{}{}
			objs = append(objs, ObjectInfo{Key: v.Key, Size: v.Size, LastModified: v.LastModified, ETag: v.ETag})
		}
		return objs, "", false, nil
	}
	objs, next, truncated, err := s.objects.ListObjects(bucket, prefix, continuationToken, maxKeys)
	if err != nil {
		return nil, "", false, err
	}
	for i := range objs {
		etag, err := s.meta.GetObjectMeta(bucket, objs[i].Key)
		if err == nil {
			objs[i].ETag = etag
		}
	}
	return objs, next, truncated, nil
}

func (s *Store) replicatePut(bucket, key string) {
	if s.replicationDir == "" {
		return
	}
	rc, _, err := s.GetObjectVersion(bucket, key, "")
	if err != nil {
		return
	}
	defer rc.Close()
	clean, ok := cleanKey(key)
	if !ok {
		return
	}
	p := filepath.Join(s.replicationDir, bucket, clean)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.Create(p)
	if err != nil {
		return
	}
	_, _ = io.Copy(f, rc)
	_ = f.Close()
}

func (s *Store) replicateDelete(bucket, key string) {
	if s.replicationDir == "" {
		return
	}
	clean, ok := cleanKey(key)
	if !ok {
		return
	}
	_ = os.Remove(filepath.Join(s.replicationDir, bucket, clean))
}

// ---- Local filesystem implementations ----

type fsMetadataStore struct {
	root string
}

type objectMeta struct {
	ETag string `json:"etag"`
}

func newFSMetadataStore(root string) *fsMetadataStore { return &fsMetadataStore{root: root} }

func (m *fsMetadataStore) bucketDir(bucket string) string { return filepath.Join(m.root, bucket) }
func (m *fsMetadataStore) objectsDir(bucket string) string {
	return filepath.Join(m.bucketDir(bucket), "objects")
}

func (m *fsMetadataStore) ListBuckets() ([]BucketInfo, error) {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return nil, err
	}
	out := make([]BucketInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		st, err := os.Stat(filepath.Join(m.root, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, BucketInfo{Name: e.Name(), CreationDate: st.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *fsMetadataStore) BucketExists(bucket string) (bool, error) {
	st, err := os.Stat(m.objectsDir(bucket))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return st.IsDir(), nil
}

func (m *fsMetadataStore) CreateBucket(bucket string) error {
	if !validateBucket(bucket) {
		return errors.New("invalid bucket name")
	}
	return os.MkdirAll(m.objectsDir(bucket), 0o755)
}

func (m *fsMetadataStore) DeleteBucket(bucket string) error {
	objDir := m.objectsDir(bucket)
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
		if !info.IsDir() && !strings.HasSuffix(path, ".meta.json") {
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
	return os.RemoveAll(m.bucketDir(bucket))
}

func metaPath(path string) string { return path + ".meta.json" }

func (m *fsMetadataStore) PutObjectMeta(bucket, key, etag string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	p := filepath.Join(m.objectsDir(bucket), clean)
	b, err := json.Marshal(objectMeta{ETag: etag})
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(p), b, 0o644)
}

func (m *fsMetadataStore) GetObjectMeta(bucket, key string) (string, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return "", ErrObjectNotFound
	}
	p := filepath.Join(m.objectsDir(bucket), clean)
	b, err := os.ReadFile(metaPath(p))
	if err != nil {
		return "", err
	}
	var om objectMeta
	if err := json.Unmarshal(b, &om); err != nil {
		return "", err
	}
	return om.ETag, nil
}

func (m *fsMetadataStore) DeleteObjectMeta(bucket, key string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	p := filepath.Join(m.objectsDir(bucket), clean)
	if err := os.Remove(metaPath(p)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type fsObjectBackend struct {
	root string
}

func newFSObjectBackend(root string) *fsObjectBackend { return &fsObjectBackend{root: root} }

func (b *fsObjectBackend) objectsDir(bucket string) string {
	return filepath.Join(b.root, bucket, "objects")
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

func (b *fsObjectBackend) PutObject(bucket, key string, r io.Reader) (ObjectInfo, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return ObjectInfo{}, errors.New("invalid key")
	}
	if _, err := os.Stat(b.objectsDir(bucket)); err != nil {
		if os.IsNotExist(err) {
			return ObjectInfo{}, ErrBucketNotFound
		}
		return ObjectInfo{}, err
	}
	p := filepath.Join(b.objectsDir(bucket), clean)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return ObjectInfo{}, err
	}
	f, err := os.Create(p)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer f.Close()
	h := md5.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	if err != nil {
		return ObjectInfo{}, err
	}
	st, _ := f.Stat()
	return ObjectInfo{Key: key, Size: n, LastModified: st.ModTime().UTC(), ETag: `"` + hex.EncodeToString(h.Sum(nil)) + `"`}, nil
}

func (b *fsObjectBackend) GetObject(bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	p := filepath.Join(b.objectsDir(bucket), clean)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ObjectInfo{}, ErrObjectNotFound
		}
		return nil, ObjectInfo{}, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, ObjectInfo{}, err
	}
	return f, ObjectInfo{Key: key, Size: st.Size(), LastModified: st.ModTime().UTC()}, nil
}

func (b *fsObjectBackend) DeleteObject(bucket, key string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	p := filepath.Join(b.objectsDir(bucket), clean)
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return err
	}
	return nil
}

func (b *fsObjectBackend) ListObjects(bucket, prefix, continuationToken string, maxKeys int) ([]ObjectInfo, string, bool, error) {
	root := b.objectsDir(bucket)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, "", false, ErrBucketNotFound
		}
		return nil, "", false, err
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	all := make([]ObjectInfo, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || strings.HasSuffix(path, ".meta.json") {
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
		if continuationToken != "" && key <= continuationToken {
			return nil
		}
		all = append(all, ObjectInfo{Key: key, Size: info.Size(), LastModified: info.ModTime().UTC()})
		return nil
	})
	if err != nil {
		return nil, "", false, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Key < all[j].Key })
	if len(all) <= maxKeys {
		return all, "", false, nil
	}
	page := all[:maxKeys]
	next := page[len(page)-1].Key
	return page, next, true, nil
}
