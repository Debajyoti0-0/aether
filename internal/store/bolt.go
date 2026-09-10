package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names used by aether.
var (
	BucketTokens  = []byte("tokens")
	BucketRuns    = []byte("runs")
	BucketResults = []byte("results")
)

// Store wraps a BoltDB key-value database.
type Store struct {
	db *bolt.DB
}

// OpenStore opens (creating if necessary) the database at path.
func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store path is required")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create store dir %s: %w", dir, err)
		}
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", path, err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{BucketTokens, BucketRuns, BucketResults} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// PutJSON stores any value as JSON under bucket/key.
func (s *Store) PutJSON(bucket []byte, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// GetJSON loads a JSON value from bucket/key into out.
func (s *Store) GetJSON(bucket []byte, key string, out any) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key %q not found in %q", key, bucket)
		}
		return json.Unmarshal(data, out)
	})
}

// List returns all JSON values in a bucket keyed by their keys.
func (s *Store) List(bucket []byte) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			out[string(k)] = append(json.RawMessage(nil), v...)
			return nil
		})
	})
	return out, err
}

// Delete removes a key from a bucket.
func (s *Store) Delete(bucket []byte, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}
