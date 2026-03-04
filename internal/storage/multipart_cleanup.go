package storage

import (
	"os"
	"path/filepath"
	"time"
)

func (s *Store) CleanupStaleMultipartUploads(maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}
	fs, ok := s.objects.(*fsObjectBackend)
	if !ok {
		return 0, nil
	}
	entries, err := os.ReadDir(fs.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	now := time.Now()
	for _, bucket := range entries {
		if !bucket.IsDir() {
			continue
		}
		mpDir := filepath.Join(fs.root, bucket.Name(), ".multipart")
		uploads, err := os.ReadDir(mpDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		for _, up := range uploads {
			if !up.IsDir() {
				continue
			}
			st, err := up.Info()
			if err != nil {
				continue
			}
			if now.Sub(st.ModTime()) > maxAge {
				if err := os.RemoveAll(filepath.Join(mpDir, up.Name())); err == nil {
					removed++
				}
			}
		}
	}
	return removed, nil
}
