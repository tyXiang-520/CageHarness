package memory

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileStore is a JSON-file-based persistence store for MemoryEntry objects.
// It stores entries as a JSON array in a file at the given path.
type FileStore struct {
	path    string
	entries []MemoryEntry
}

// NewFileStore creates a new FileStore backed by the given file path.
// If the file exists, it loads existing entries. Otherwise, it creates an empty store.
func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:    path,
		entries: make([]MemoryEntry, 0),
	}

	// Try to load existing data
	if _, err := os.Stat(path); err == nil {
		if err := store.loadFromDisk(); err != nil {
			return nil, fmt.Errorf("load existing memory store: %w", err)
		}
	} else {
		// Create empty file
		if err := store.writeToDisk(); err != nil {
			return nil, fmt.Errorf("create memory store: %w", err)
		}
	}

	return store, nil
}

// Save persists a MemoryEntry to the store.
func (s *FileStore) Save(entry MemoryEntry) error {
	s.entries = append(s.entries, entry)
	return s.writeToDisk()
}

// Load returns all entries in the store.
func (s *FileStore) Load() ([]MemoryEntry, error) {
	result := make([]MemoryEntry, len(s.entries))
	copy(result, s.entries)
	return result, nil
}

// Get retrieves a MemoryEntry by ID.
func (s *FileStore) Get(id string) (MemoryEntry, bool) {
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return MemoryEntry{}, false
}

// Delete removes a MemoryEntry by ID.
// Returns nil even if the ID doesn't exist (idempotent).
func (s *FileStore) Delete(id string) error {
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return s.writeToDisk()
		}
	}
	// Not found — idempotent, no error
	return nil
}

// loadFromDisk reads the JSON file into memory.
func (s *FileStore) loadFromDisk() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		s.entries = make([]MemoryEntry, 0)
		return nil
	}

	return json.Unmarshal(data, &s.entries)
}

// writeToDisk writes the current entries to the JSON file.
func (s *FileStore) writeToDisk() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal entries: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Len returns the number of entries in the store.
func (s *FileStore) Len() int {
	return len(s.entries)
}