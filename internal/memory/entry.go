package memory

import (
	"fmt"
	"time"
)

// MemoryType represents the type of memory storage.
type MemoryType int

const (
	// MemoryTypeEphemeral: short-term, session-scoped memory (conversation context).
	MemoryTypeEphemeral MemoryType = iota
	// MemoryTypeLongTerm: persistent memory stored across sessions.
	MemoryTypeLongTerm
	// MemoryTypeWorking: intermediate working memory for the current decision cycle.
	MemoryTypeWorking
)

// String returns the human-readable name of the memory type.
func (mt MemoryType) String() string {
	switch mt {
	case MemoryTypeEphemeral:
		return "ephemeral"
	case MemoryTypeLongTerm:
		return "long_term"
	case MemoryTypeWorking:
		return "working"
	default:
		return fmt.Sprintf("MemoryType(%d)", int(mt))
	}
}

// MemoryEntry represents a single unit of memory stored by the Agent.
type MemoryEntry struct {
	ID         string         `json:"id"`
	Type       MemoryType     `json:"type"`
	Content    string         `json:"content"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	AccessedAt time.Time      `json:"accessed_at"`
	TTL        time.Duration  `json:"ttl_ns,omitempty"`
}

// NewMemoryEntry creates a new MemoryEntry with a generated ID.
func NewMemoryEntry(mt MemoryType, content string, metadata map[string]any) MemoryEntry {
	now := time.Now()
	return MemoryEntry{
		ID:         generateMemoryID(),
		Type:       mt,
		Content:    content,
		Metadata:   metadata,
		Timestamp:  now,
		AccessedAt: now,
	}
}

// IsExpired returns true if the entry has a TTL set and the current time
// is past the expiry time (Timestamp + TTL).
func (e MemoryEntry) IsExpired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.Timestamp) > e.TTL
}

// generateMemoryID generates a unique memory entry ID.
func generateMemoryID() string {
	return fmt.Sprintf("mem-%d", time.Now().UnixNano())
}