package memory

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryEntry_ZeroValue(t *testing.T) {
	var e MemoryEntry
	if e.ID != "" {
		t.Errorf("zero value of MemoryEntry.ID should be empty, got %q", e.ID)
	}
	if e.Type != MemoryTypeEphemeral {
		t.Errorf("zero value of MemoryEntry.Type should be Ephemeral, got %v", e.Type)
	}
}

func TestMemoryEntry_NewMemoryEntry(t *testing.T) {
	entry := NewMemoryEntry(MemoryTypeEphemeral, "test content", nil)
	if entry.ID == "" {
		t.Error("NewMemoryEntry should generate a non-empty ID")
	}
	if entry.Content != "test content" {
		t.Errorf("Content = %q, want %q", entry.Content, "test content")
	}
	if entry.Type != MemoryTypeEphemeral {
		t.Errorf("Type = %v, want %v", entry.Type, MemoryTypeEphemeral)
	}
	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestMemoryEntry_WithMetadata(t *testing.T) {
	meta := map[string]any{"source": "agent", "priority": 1}
	entry := NewMemoryEntry(MemoryTypeLongTerm, "important data", meta)
	if entry.Metadata["source"] != "agent" {
		t.Errorf("Metadata[source] = %v, want %v", entry.Metadata["source"], "agent")
	}
	if entry.Metadata["priority"] != 1 {
		t.Errorf("Metadata[priority] = %v (type: %T), want %v", entry.Metadata["priority"], entry.Metadata["priority"], 1)
	}
}

func TestMemoryEntry_IsExpired(t *testing.T) {
	t.Run("no TTL", func(t *testing.T) {
		entry := NewMemoryEntry(MemoryTypeEphemeral, "data", nil)
		if entry.IsExpired() {
			t.Error("entry without TTL should not be expired")
		}
	})

	t.Run("expired entry", func(t *testing.T) {
		entry := MemoryEntry{
			ID:        "mem-1",
			Type:      MemoryTypeEphemeral,
			Content:   "stale",
			Timestamp: time.Now().Add(-10 * time.Minute),
			TTL:       5 * time.Minute,
		}
		if !entry.IsExpired() {
			t.Error("entry past TTL should be expired")
		}
	})

	t.Run("valid entry", func(t *testing.T) {
		entry := MemoryEntry{
			ID:        "mem-2",
			Type:      MemoryTypeEphemeral,
			Content:   "fresh",
			Timestamp: time.Now().Add(-1 * time.Minute),
			TTL:       30 * time.Minute,
		}
		if entry.IsExpired() {
			t.Error("entry within TTL should not be expired")
		}
	})
}

func TestMemoryEntry_JSONRoundTrip(t *testing.T) {
	meta := map[string]any{"key": "value"}
	original := NewMemoryEntry(MemoryTypeLongTerm, "json test", meta)
	original.TTL = 10 * time.Minute

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded MemoryEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, original.Content)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, original.Type)
	}
}

func TestMemoryType_String(t *testing.T) {
	tests := []struct {
		mt   MemoryType
		want string
	}{
		{MemoryTypeEphemeral, "ephemeral"},
		{MemoryTypeLongTerm, "long_term"},
		{MemoryTypeWorking, "working"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mt.String(); got != tt.want {
				t.Errorf("MemoryType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}