package memory

import (
	"sort"
	"strings"
	"time"
)

// Retriever searches MemoryEntry objects by keyword matching.
type Retriever struct {
	store *FileStore
}

// NewRetriever creates a new Retriever backed by the given store.
func NewRetriever(store *FileStore) *Retriever {
	return &Retriever{store: store}
}

// scoredEntry pairs a MemoryEntry with its match score.
type scoredEntry struct {
	entry MemoryEntry
	score int
}

// Retrieve finds entries matching the query string, sorted by relevance.
// Returns up to limit results. Empty query returns empty slice.
func (r *Retriever) Retrieve(query string, limit int) []MemoryEntry {
	if query == "" || limit <= 0 {
		return nil
	}

	entries, err := r.store.Load()
	if err != nil {
		return nil
	}

	query = strings.ToLower(query)
	queryWords := strings.Fields(query)

	var scored []scoredEntry
	for _, entry := range entries {
		score := r.scoreEntry(entry, query, queryWords)
		if score > 0 {
			scored = append(scored, scoredEntry{entry: entry, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Apply limit
	if limit > len(scored) {
		limit = len(scored)
	}

	results := make([]MemoryEntry, limit)
	for i := 0; i < limit; i++ {
		results[i] = scored[i].entry
		// Update AccessedAt
		results[i].AccessedAt = time.Now()
	}

	return results
}

// scoreEntry computes a relevance score for an entry against the query.
func (r *Retriever) scoreEntry(entry MemoryEntry, fullQuery string, queryWords []string) int {
	score := 0

	contentLower := strings.ToLower(entry.Content)
	tagsLower := ""
	if tags, ok := entry.Metadata["tags"]; ok {
		if tagsStr, ok := tags.(string); ok {
			tagsLower = strings.ToLower(tagsStr)
		}
	}

	// Full query match in content (high weight)
	if strings.Contains(contentLower, fullQuery) {
		score += 10
	}

	// Full query match in tags (high weight)
	if tagsLower != "" && strings.Contains(tagsLower, fullQuery) {
		score += 8
	}

	// Individual word matches
	for _, word := range queryWords {
		if word == "" {
			continue
		}
		// Content word match
		if strings.Contains(contentLower, word) {
			score += 2
		}
		// Tag word match
		if tagsLower != "" && strings.Contains(tagsLower, word) {
			score += 3
		}
	}

	return score
}