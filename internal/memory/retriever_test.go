package memory

import (
	"testing"
)

func TestRetriever_ExactMatch(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "Use camelCase for JS variables", map[string]any{
		"tags": "js,naming,convention",
	})
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("js naming convention", 5)

	if len(results) == 0 {
		t.Error("should find the entry with matching tags")
	}
	if len(results) > 0 && results[0].ID != entry.ID {
		t.Errorf("expected entry %s, got %s", entry.ID, results[0].ID)
	}
}

func TestRetriever_ContentMatch(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "Always use const for immutable variables", map[string]any{
		"tags": "javascript",
	})
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("immutable variables", 5)

	if len(results) == 0 {
		t.Error("should find entry by content match")
	}
}

func TestRetriever_NoMatch(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "Python specific stuff", map[string]any{
		"tags": "python",
	})
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("golang concurrency", 5)

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRetriever_EmptyStore(t *testing.T) {
	store, _ := tempStore(t)
	retriever := NewRetriever(store)

	results := retriever.Retrieve("anything", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestRetriever_EmptyQuery(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "some content", nil)
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("", 5)

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestRetriever_Limit(t *testing.T) {
	store, _ := tempStore(t)

	// Save 5 entries with similar content
	for i := 0; i < 5; i++ {
		entry := NewMemoryEntry(MemoryTypeLongTerm, "go test pattern", map[string]any{
			"tags": "go,testing",
		})
		if err := store.Save(entry); err != nil {
			t.Fatalf("Save[%d]: %v", i, err)
		}
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("go testing", 3)

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestRetriever_TagMatch(t *testing.T) {
	store, _ := tempStore(t)

	entry1 := NewMemoryEntry(MemoryTypeLongTerm, "Go naming", map[string]any{
		"tags": "go,convention",
	})
	entry2 := NewMemoryEntry(MemoryTypeLongTerm, "Python naming", map[string]any{
		"tags": "python,convention",
	})
	if err := store.Save(entry1); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Save(entry2); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)

	// Query for "go" should match entry1
	results := retriever.Retrieve("go", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'go' query, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != entry1.ID {
		t.Errorf("expected entry1, got %s", results[0].ID)
	}

	// Query for "convention" should match both
	results = retriever.Retrieve("convention", 5)
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'convention' query, got %d", len(results))
	}
}

func TestRetriever_MatchScore(t *testing.T) {
	store, _ := tempStore(t)

	// Entry with exact match in both tags and content
	exact := NewMemoryEntry(MemoryTypeLongTerm, "golang error handling best practices", map[string]any{
		"tags": "golang,error,handling",
	})
	// Entry with partial match
	partial := NewMemoryEntry(MemoryTypeLongTerm, "general programming tips", map[string]any{
		"tags": "golang",
	})

	if err := store.Save(partial); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Save(exact); err != nil {
		t.Fatalf("Save: %v", err)
	}

	retriever := NewRetriever(store)
	results := retriever.Retrieve("golang error handling", 5)

	// The exact match should come first (higher score)
	if len(results) >= 2 && results[0].ID != exact.ID {
		t.Errorf("expected exact match first, got %s", results[0].ID)
	}
}

func TestRetriever_UpdatesAccessedAt(t *testing.T) {
	store, _ := tempStore(t)

	entry := NewMemoryEntry(MemoryTypeLongTerm, "test content", map[string]any{
		"tags": "test",
	})
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	originalAccessedAt := entry.AccessedAt

	retriever := NewRetriever(store)
	results := retriever.Retrieve("test", 5)

	if len(results) > 0 && results[0].AccessedAt.Before(originalAccessedAt) {
		t.Error("AccessedAt should be updated after retrieval")
	}
}