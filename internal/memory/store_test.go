package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store, path
}

func TestFileStore_Save(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "Use snake_case for Go variables", map[string]any{
		"tags":    "go,naming,convention",
		"priority": 1,
	})

	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the entry was saved
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded))
	}
	if loaded[0].Content != "Use snake_case for Go variables" {
		t.Errorf("unexpected content: %s", loaded[0].Content)
	}
}

func TestFileStore_SaveMultiple(t *testing.T) {
	store, _ := tempStore(t)

	for i := 0; i < 5; i++ {
		entry := NewMemoryEntry(MemoryTypeLongTerm, "content", nil)
		if err := store.Save(entry); err != nil {
			t.Fatalf("Save[%d]: %v", i, err)
		}
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 5 {
		t.Errorf("expected 5 entries, got %d", len(loaded))
	}
}

func TestFileStore_Get(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeEphemeral, "test content", nil)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Get existing entry
	found, ok := store.Get(entry.ID)
	if !ok {
		t.Error("Get should find the entry")
	}
	if found.Content != "test content" {
		t.Errorf("unexpected content: %s", found.Content)
	}

	// Get non-existent entry
	_, ok = store.Get("nonexistent")
	if ok {
		t.Error("Get should not find nonexistent entry")
	}
}

func TestFileStore_Delete(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "to delete", nil)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Delete existing
	if err := store.Delete(entry.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify deleted
	_, ok := store.Get(entry.ID)
	if ok {
		t.Error("Get should not find deleted entry")
	}

	// Delete non-existent should not error
	if err := store.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent should not error: %v", err)
	}
}

func TestFileStore_EmptyStore(t *testing.T) {
	store, _ := tempStore(t)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load empty store: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded))
	}
}

func TestFileStore_NewStoreCreatesFile(t *testing.T) {
	_, path := tempStore(t)

	// File should exist after store creation
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("store file should exist after creation")
	}
}

func TestFileStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	// Create and save
	store1, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	entry := NewMemoryEntry(MemoryTypeLongTerm, "persistent content", map[string]any{
		"tags": "persistence",
	})
	if err := store1.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Re-open and verify
	store2, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore (reopen): %v", err)
	}
	loaded, err := store2.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry after reopen, got %d", len(loaded))
	}
	if loaded[0].Content != "persistent content" {
		t.Errorf("unexpected content: %s", loaded[0].Content)
	}
}

func TestFileStore_JSONFormat(t *testing.T) {
	store, path := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "json test", map[string]any{
		"tags": "test",
	})
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw file to verify JSON format
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Error("file should not be empty")
	}
	// Should be a valid JSON array
	if content[0] != '[' {
		t.Errorf("expected JSON array, got: %s", content[:50])
	}
}

func TestFileStore_IsExpired(t *testing.T) {
	store, _ := tempStore(t)

	// Non-expired entry (no TTL)
	entry := NewMemoryEntry(MemoryTypeEphemeral, "permanent", nil)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("non-expired entry should be loaded")
	}
}