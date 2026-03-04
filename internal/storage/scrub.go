package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		norm := filepath.ToSlash(path)

		if strings.HasSuffix(norm, ".meta.json") {
			obj := strings.TrimSuffix(path, ".meta.json")
			if _, err := os.Stat(obj); err != nil {
				report.OrphanMeta++
				if repair {
					_ = os.Remove(path)
				}
			}
			return nil
		}

		// Only scrub primary object files under /objects/; skip multipart/versioning internals.
		if !strings.Contains(norm, "/objects/") {
			return nil
		}

		report.CheckedObjects++
		metaPath := path + ".meta.json"
		if _, err := os.Stat(metaPath); err == nil {
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
		b, err := json.Marshal(objectMeta{ETag: etag})
		if err != nil {
			return nil
		}
		_ = os.WriteFile(metaPath, b, 0o644)
		return nil
	})
	return report, err
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
