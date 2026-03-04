package storage

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

var (
	bucketInfoBucket = []byte("bucket_info")
	objectMetaBucket = []byte("object_meta")
)

type bboltMetadataStore struct {
	db *bbolt.DB
}

func newBboltMetadataStore(db *bbolt.DB) (*bboltMetadataStore, error) {
	err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketInfoBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(objectMetaBucket); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &bboltMetadataStore{db: db}, nil
}

func (m *bboltMetadataStore) ListBuckets() ([]BucketInfo, error) {
	var out []BucketInfo
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInfoBucket)
		return b.ForEach(func(k, v []byte) error {
			var t time.Time
			if err := t.UnmarshalText(v); err == nil {
				out = append(out, BucketInfo{Name: string(k), CreationDate: t.UTC()})
			}
			return nil
		})
	})
	return out, err
}

func (m *bboltMetadataStore) BucketExists(bucket string) (bool, error) {
	if !validateBucket(bucket) {
		return false, nil
	}
	var exists bool
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInfoBucket)
		exists = b.Get([]byte(bucket)) != nil
		return nil
	})
	return exists, err
}

func (m *bboltMetadataStore) CreateBucket(bucket string) error {
	if !validateBucket(bucket) {
		return errors.New("invalid bucket name")
	}
	return m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInfoBucket)
		if b.Get([]byte(bucket)) != nil {
			return nil // already exists
		}
		t, _ := time.Now().UTC().MarshalText()
		return b.Put([]byte(bucket), t)
	})
}

func (m *bboltMetadataStore) DeleteBucket(bucket string) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		bInfo := tx.Bucket(bucketInfoBucket)
		if bInfo.Get([]byte(bucket)) == nil {
			return ErrBucketNotFound
		}

		// check if bucket empty by scanning object meta
		bMeta := tx.Bucket(objectMetaBucket)
		prefix := []byte(bucket + "/")
		c := bMeta.Cursor()
		k, _ := c.Seek(prefix)
		if k != nil && strings.HasPrefix(string(k), string(prefix)) {
			return ErrBucketNotEmpty
		}

		return bInfo.Delete([]byte(bucket))
	})
}

func (m *bboltMetadataStore) PutObjectMeta(bucket, key, etag string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	fullKey := []byte(bucket + "/" + clean)
	b, err := json.Marshal(objectMeta{ETag: etag})
	if err != nil {
		return err
	}
	return m.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(objectMetaBucket).Put(fullKey, b)
	})
}

func (m *bboltMetadataStore) GetObjectMeta(bucket, key string) (string, error) {
	clean, ok := cleanKey(key)
	if !ok {
		return "", ErrObjectNotFound
	}
	fullKey := []byte(bucket + "/" + clean)
	var etag string
	err := m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(objectMetaBucket)
		val := b.Get(fullKey)
		if val == nil {
			return ErrObjectNotFound
		}
		var om objectMeta
		if err := json.Unmarshal(val, &om); err != nil {
			return err
		}
		etag = om.ETag
		return nil
	})
	return etag, err
}

func (m *bboltMetadataStore) DeleteObjectMeta(bucket, key string) error {
	clean, ok := cleanKey(key)
	if !ok {
		return ErrObjectNotFound
	}
	fullKey := []byte(bucket + "/" + clean)
	return m.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(objectMetaBucket).Delete(fullKey)
	})
}
