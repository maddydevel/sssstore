package storage

import (
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
		if strings.HasSuffix(path, ".meta.json") {
			obj := strings.TrimSuffix(path, ".meta.json")
			if _, err := os.Stat(obj); err != nil {
				report.OrphanMeta++
				if repair {
					_ = os.Remove(path)
				}
			}
			return nil
		}
		if strings.Contains(path, "/versions/") && strings.HasSuffix(path, ".data") {
			return nil
		}
		report.CheckedObjects++
		if _, err := os.Stat(path + ".meta.json"); err != nil {
			report.MissingMeta++
			if repair {
				_ = s.meta.PutObjectMeta(bucketFromPath(root, path), keyFromPath(path), "")
			}
		}
		return nil
	})
	return report, err
}

func bucketFromPath(root, p string) string {
	rel, _ := filepath.Rel(root, p)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func keyFromPath(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/objects/")
	if len(parts) == 2 {
		return parts[1]
	}
	return filepath.Base(p)
}
