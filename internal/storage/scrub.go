package storage

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.etcd.io/bbolt"
)

type ScrubReport struct {
	CheckedObjects int
	MissingMeta    int
	OrphanMeta     int
}

func (s *Store) Scrub(repair bool) (ScrubReport, error) {
	root, ok := s.fsRoot()
	if !ok {
		return ScrubReport{}, nil
	}
	report := ScrubReport{}

	// 1. Find all physical objects and ensure they have metadata
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		norm := filepath.ToSlash(path)

		// Only scrub primary object files under /objects/; skip multipart/versioning internals
		// and any lingering legacy .meta.json files
		if !strings.Contains(norm, "/objects/") || strings.HasSuffix(norm, ".meta.json") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 3 {
			return nil // e.g. metadata.db file
		}
		bucket := parts[0]
		key := strings.Join(parts[2:], "/")

		report.CheckedObjects++

		_, err = s.meta.GetObjectMeta(bucket, key)
		if err == nil {
			return nil
		}

		report.MissingMeta++
		if !repair {
			return nil
		}

		etag, err := computeFileETag(path)
		if err != nil {
			return nil
		}
		_ = s.meta.PutObjectMeta(bucket, key, etag)
		return nil
	})
	if err != nil {
		return report, err
	}

	// 2. Find orphan metadata (metadata exists but no physical object)
	if boltMeta, ok := s.meta.(*bboltMetadataStore); ok {
		_ = boltMeta.db.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket(objectMetaBucket)
			if b == nil {
				return nil
			}
			return b.ForEach(func(k, v []byte) error {
				parts := strings.SplitN(string(k), "/", 2)
				if len(parts) != 2 {
					return nil
				}
				bucket := parts[0]
				key := parts[1]

				objPath := filepath.Join(root, bucket, "objects", filepath.FromSlash(key))
				if _, err := os.Stat(objPath); err != nil {
					report.OrphanMeta++
				}
				return nil
			})
		})
	}

	if repair && report.OrphanMeta > 0 {
		if boltMeta, ok := s.meta.(*bboltMetadataStore); ok {
			_ = boltMeta.db.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket(objectMetaBucket)
				if b == nil {
					return nil
				}
				c := b.Cursor()
				for k, _ := c.First(); k != nil; k, _ = c.Next() {
					parts := strings.SplitN(string(k), "/", 2)
					if len(parts) != 2 {
						continue
					}
					bucket := parts[0]
					key := parts[1]

					objPath := filepath.Join(root, bucket, "objects", filepath.FromSlash(key))
					if _, err := os.Stat(objPath); err != nil {
						_ = c.Delete()
					}
				}
				return nil
			})
		}
	}

	return report, nil
}

func computeFileETag(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return `"` + hex.EncodeToString(h.Sum(nil)) + `"`, nil
}
