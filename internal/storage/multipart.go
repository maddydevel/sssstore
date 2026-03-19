package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MultipartPart struct {
	PartNumber int
	ETag       string
	Size       int64
}

func (s *Store) multipartDir(bucket string) string {
	if fs, ok := s.objects.(*fsObjectBackend); ok {
		return filepath.Join(fs.root, bucket, ".multipart")
	}
	return ""
}

func (s *Store) CreateMultipartUpload(bucket, key string) (string, error) {
	exists, err := s.BucketExists(bucket)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrBucketNotFound
	}
	uploadID := fmt.Sprintf("%d", time.Now().UnixNano())
	p := filepath.Join(s.multipartDir(bucket), uploadID)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(p, "key"), []byte(key), 0o644); err != nil {
		return "", err
	}
	return uploadID, nil
}

func (s *Store) UploadPart(bucket, key, uploadID string, partNumber int, r io.Reader) (MultipartPart, error) {
	if partNumber < 1 {
		return MultipartPart{}, ErrObjectNotFound
	}
	p := filepath.Join(s.multipartDir(bucket), uploadID)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return MultipartPart{}, ErrObjectNotFound
		}
		return MultipartPart{}, err
	}
	partPath := filepath.Join(p, fmt.Sprintf("part-%05d", partNumber))
	f, err := os.Create(partPath)
	if err != nil {
		return MultipartPart{}, err
	}
	defer f.Close()
	h := md5.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	if err != nil {
		return MultipartPart{}, err
	}
	etag := `"` + hex.EncodeToString(h.Sum(nil)) + `"`
	if err := os.WriteFile(partPath+".etag", []byte(etag), 0o644); err != nil {
		return MultipartPart{}, err
	}
	return MultipartPart{PartNumber: partNumber, ETag: etag, Size: n}, nil
}

func (s *Store) CompleteMultipartUpload(bucket, key, uploadID string) (ObjectInfo, error) {
	p := filepath.Join(s.multipartDir(bucket), uploadID)
	entries, err := os.ReadDir(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, err
	}
	parts := make([]string, 0)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "part-") && !strings.HasSuffix(name, ".etag") {
			parts = append(parts, name)
		}
	}
	sort.Strings(parts)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for _, pn := range parts {
			f, err := os.Open(filepath.Join(p, pn))
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			_, err = io.Copy(pw, f)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()
	info, err := s.PutObject(bucket, key, pr)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := os.RemoveAll(p); err != nil {
		return info, err
	}
	return info, nil
}

func (s *Store) AbortMultipartUpload(bucket, uploadID string) error {
	p := filepath.Join(s.multipartDir(bucket), uploadID)
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	return nil
}

func parsePartNumber(v string) (int, error) {
	return strconv.Atoi(v)
}
