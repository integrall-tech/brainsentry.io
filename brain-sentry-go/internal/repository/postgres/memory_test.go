package postgres

import (
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
)

func TestMemoryColumns_ContainsDeletedAt(t *testing.T) {
	if !containsSubstring(memoryColumns, "deleted_at") {
		t.Error("memoryColumns should contain deleted_at for soft delete support")
	}
}

func TestMemoryColumns_ContainsEmotionalWeight(t *testing.T) {
	if !containsSubstring(memoryColumns, "emotional_weight") {
		t.Error("memoryColumns should contain emotional_weight")
	}
}

func TestMemoryColumns_ContainsSimHash(t *testing.T) {
	if !containsSubstring(memoryColumns, "sim_hash") {
		t.Error("memoryColumns should contain sim_hash")
	}
}

func TestMemoryColumns_HasCorrectCount(t *testing.T) {
	// 32 columns total: original set + temporal decay/supersession +
	// bi-temporal recorded_at + provenance (migration 000010).
	expected := 32
	count := countColumns(memoryColumns)
	if count != expected {
		t.Errorf("expected %d columns, got %d", expected, count)
	}
}

func TestMemoryToJSON_Nil(t *testing.T) {
	result := MemoryToJSON(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestMemoryToJSON_ValidMap(t *testing.T) {
	data := map[string]any{"key": "value", "num": 42}
	result := MemoryToJSON(data)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestScanMemory_NilRow(t *testing.T) {
	// Passing nil to scanMemory would panic, so we just verify the function signature exists
	// Real scan testing requires pgx.Row mock or integration tests
	t.Log("scanMemory requires pgx.Row interface - tested via integration tests")
}

// Test that domain.Memory has DeletedAt field (compile-time check)
func TestMemoryDomain_HasDeletedAt(t *testing.T) {
	m := domain.Memory{}
	if m.DeletedAt != nil {
		t.Error("new memory should have nil DeletedAt")
	}
}

func TestMemoryDomain_HasEmotionalWeight(t *testing.T) {
	m := domain.Memory{EmotionalWeight: 0.5}
	if m.EmotionalWeight != 0.5 {
		t.Errorf("expected 0.5, got %f", m.EmotionalWeight)
	}
}

func TestMemoryDomain_HasSimHash(t *testing.T) {
	m := domain.Memory{SimHash: "abc123"}
	if m.SimHash != "abc123" {
		t.Errorf("expected 'abc123', got '%s'", m.SimHash)
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func countColumns(cols string) int {
	count := 1
	for _, c := range cols {
		if c == ',' {
			count++
		}
	}
	return count
}
