// Package memory provides a simple, durable key/value store for agent
// memories, backed by etcd's bbolt embedded database. Each memory is a plain
// string value addressed by a string key.
//
// Example usage:
//
//	store, err := memory.NewStore("memories.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer store.Close()
//
//	store.Put("last_objective", "refactor the planner")
//	value, err := store.Get("last_objective")
package memory

import (
	"errors"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// ErrNotFound is returned when a requested key does not exist in the store.
var ErrNotFound = errors.New("memory: key not found")

// ErrEmptyKey is returned when an operation is attempted with an empty key.
var ErrEmptyKey = errors.New("memory: key must not be empty")

// bucketName is the single bbolt bucket used to hold all memories.
var bucketName = []byte("memories")

// Store is a durable key/value store for agent memories.
type Store struct {
	db *bolt.DB
}

// NewStore opens (creating if necessary) a bbolt database at the given path
// and ensures the memories bucket exists, leaving the returned Store ready
// for immediate use.
func NewStore(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not NewStore: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("could not NewStore: %v", err)
	}

	return &Store{db: db}, nil
}

// Put stores value under key, overwriting any existing value for that key.
func (s *Store) Put(key, value string) error {
	if key == "" {
		return ErrEmptyKey
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(key), []byte(value))
	})
	if err != nil {
		return fmt.Errorf("could not Store.Put: %v", err)
	}

	return nil
}

// Get returns the value stored under key, or ErrNotFound if no such key
// exists.
func (s *Store) Get(key string) (string, error) {
	if key == "" {
		return "", ErrEmptyKey
	}

	var value string

	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketName).Get([]byte(key))
		if data == nil {
			return ErrNotFound
		}

		value = string(data)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFound
		}

		return "", fmt.Errorf("could not Store.Get: %v", err)
	}

	return value, nil
}

// Delete removes key from the store. Deleting a key that does not exist is
// not an error.
func (s *Store) Delete(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(key))
	})
	if err != nil {
		return fmt.Errorf("could not Store.Delete: %v", err)
	}

	return nil
}

// List returns every key/value pair currently in the store.
func (s *Store) List() (map[string]string, error) {
	memories := make(map[string]string)

	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).ForEach(func(k, v []byte) error {
			memories[string(k)] = string(v)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("could not Store.List: %v", err)
	}

	return memories, nil
}

// Search returns every key/value pair whose key or value contains query,
// case-insensitively. An empty query matches every memory, the same as
// List.
func (s *Store) Search(query string) (map[string]string, error) {
	memories, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("could not Store.Search: %v", err)
	}

	if query == "" {
		return memories, nil
	}

	query = strings.ToLower(query)
	matches := make(map[string]string)

	for key, value := range memories {
		if strings.Contains(strings.ToLower(key), query) || strings.Contains(strings.ToLower(value), query) {
			matches[key] = value
		}
	}

	return matches, nil
}

// Close releases the underlying database file.
func (s *Store) Close() error {
	err := s.db.Close()
	if err != nil {
		return fmt.Errorf("could not Store.Close: %v", err)
	}

	return nil
}
