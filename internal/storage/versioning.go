package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type bucketVersioning struct {
	Enabled bool `json:"enabled"`
}

type versionMeta struct {
	ETag         string    `json:"etag"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	DeleteMarker bool      `json:"delete_marker"`
}

type ObjectVersionInfo struct {
	Key          string    `xml:"Key"`
	VersionID    string    `xml:"VersionID"`
	IsLatest     bool      `xml:"IsLatest"`
	DeleteMarker bool      `xml:"DeleteMarker"`
	Size         int64     `xml:"Size"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag,omitempty"`
}

func (s *Store) fsRoot() (string, bool) {
	fs, ok := s.objects.(*fsObjectBackend)
	if !ok {
		return "", false
	}
	return fs.root, true
}

func (s *Store) bucketSettingsPath(bucket string) (string, error) {
	root, ok := s.fsRoot()
	if !ok {
		return "", errors.New("unsupported backend")
	}
	return filepath.Join(root, bucket, "bucket.versioning.json"), nil
}

func (s *Store) versionsBase(bucket, key string) (string, error) {
	root, ok := s.fsRoot()
	if !ok {
		return "", errors.New("unsupported backend")
	}
	clean, valid := cleanKey(key)
	if !valid {
		return "", ErrObjectNotFound
	}
	return filepath.Join(root, bucket, "versions", clean), nil
}

func (s *Store) SetBucketVersioning(bucket string, enabled bool) error {
	exists, err := s.BucketExists(bucket)
	if err != nil {
		return err
	}
	if !exists {
		return ErrBucketNotFound
	}
	p, err := s.bucketSettingsPath(bucket)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(bucketVersioning{Enabled: enabled})
	return os.WriteFile(p, b, 0o644)
}

func (s *Store) GetBucketVersioning(bucket string) (bool, error) {
	exists, err := s.BucketExists(bucket)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrBucketNotFound
	}
	p, err := s.bucketSettingsPath(bucket)
	if err != nil {
		return false, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var v bucketVersioning
	if err := json.Unmarshal(b, &v); err != nil {
		return false, err
	}
	return v.Enabled, nil
}

func versionID() string { return fmt.Sprintf("v-%d", time.Now().UnixNano()) }

func (s *Store) putObjectVersioned(bucket, key string, r io.Reader) (ObjectInfo, string, error) {
	base, err := s.versionsBase(bucket, key)
	if err != nil {
		return ObjectInfo{}, "", err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return ObjectInfo{}, "", err
	}
	vid := versionID()
	dataPath := filepath.Join(base, vid+".data")
	f, err := os.Create(dataPath)
	if err != nil {
		return ObjectInfo{}, "", err
	}
	h := md5.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	_ = f.Close()
	if err != nil {
		return ObjectInfo{}, "", err
	}
	meta := versionMeta{ETag: `"` + hex.EncodeToString(h.Sum(nil)) + `"`, Size: n, LastModified: time.Now().UTC()}
	mb, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(base, vid+".meta.json"), mb, 0o644); err != nil {
		return ObjectInfo{}, "", err
	}
	if err := os.WriteFile(filepath.Join(base, "current"), []byte(vid), 0o644); err != nil {
		return ObjectInfo{}, "", err
	}
	return ObjectInfo{Key: key, Size: n, LastModified: meta.LastModified, ETag: meta.ETag}, vid, nil
}

func (s *Store) resolveVersion(bucket, key, vid string) (versionMeta, string, error) {
	base, err := s.versionsBase(bucket, key)
	if err != nil {
		return versionMeta{}, "", err
	}
	if vid == "" {
		b, err := os.ReadFile(filepath.Join(base, "current"))
		if err != nil {
			if os.IsNotExist(err) {
				return versionMeta{}, "", ErrObjectNotFound
			}
			return versionMeta{}, "", err
		}
		vid = strings.TrimSpace(string(b))
	}
	mb, err := os.ReadFile(filepath.Join(base, vid+".meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return versionMeta{}, "", ErrObjectNotFound
		}
		return versionMeta{}, "", err
	}
	var vm versionMeta
	if err := json.Unmarshal(mb, &vm); err != nil {
		return versionMeta{}, "", err
	}
	return vm, filepath.Join(base, vid+".data"), nil
}

func (s *Store) GetObjectVersion(bucket, key, vid string) (io.ReadCloser, ObjectInfo, error) {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	if !enabled {
		return s.GetObject(bucket, key)
	}
	vm, dp, err := s.resolveVersion(bucket, key, vid)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	if vm.DeleteMarker {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	f, err := os.Open(dp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ObjectInfo{}, ErrObjectNotFound
		}
		return nil, ObjectInfo{}, err
	}
	return f, ObjectInfo{Key: key, Size: vm.Size, LastModified: vm.LastModified, ETag: vm.ETag}, nil
}

func (s *Store) DeleteObjectVersion(bucket, key, vid string) error {
	enabled, err := s.GetBucketVersioning(bucket)
	if err != nil {
		return err
	}
	if !enabled {
		return s.DeleteObject(bucket, key)
	}
	base, err := s.versionsBase(bucket, key)
	if err != nil {
		return err
	}
	if vid == "" {
		vid = versionID()
		meta := versionMeta{DeleteMarker: true, LastModified: time.Now().UTC()}
		mb, _ := json.Marshal(meta)
		if err := os.MkdirAll(base, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(base, vid+".meta.json"), mb, 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(base, "current"), []byte(vid), 0o644)
	}
	_ = os.Remove(filepath.Join(base, vid+".data"))
	_ = os.Remove(filepath.Join(base, vid+".meta.json"))
	b, err := os.ReadFile(filepath.Join(base, "current"))
	if err == nil && strings.TrimSpace(string(b)) == vid {
		_ = os.Remove(filepath.Join(base, "current"))
	}
	return nil
}

func (s *Store) ListObjectVersions(bucket, prefix string, maxKeys int) ([]ObjectVersionInfo, error) {
	exists, err := s.BucketExists(bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrBucketNotFound
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	root, ok := s.fsRoot()
	if !ok {
		return nil, errors.New("unsupported backend")
	}
	vroot := filepath.Join(root, bucket, "versions")
	out := make([]ObjectVersionInfo, 0)
	_ = filepath.Walk(vroot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		rel, _ := filepath.Rel(vroot, path)
		rel = filepath.ToSlash(rel)
		idx := strings.LastIndex(rel, "/")
		if idx < 0 {
			return nil
		}
		key := rel[:idx]
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		vid := strings.TrimSuffix(rel[idx+1:], ".meta.json")
		b, _ := os.ReadFile(path)
		var vm versionMeta
		_ = json.Unmarshal(b, &vm)
		curr, _ := os.ReadFile(filepath.Join(vroot, key, "current"))
		out = append(out, ObjectVersionInfo{Key: key, VersionID: vid, IsLatest: strings.TrimSpace(string(curr)) == vid, DeleteMarker: vm.DeleteMarker, Size: vm.Size, LastModified: vm.LastModified, ETag: vm.ETag})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key == out[j].Key {
			return out[i].LastModified.After(out[j].LastModified)
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > maxKeys {
		out = out[:maxKeys]
	}
	return out, nil
}
