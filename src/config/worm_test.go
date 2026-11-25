package config

import (
	"testing"
	"time"
)

func TestRetainUntilForSnapshot(t *testing.T) {
	snap := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	got := RetainUntilForSnapshot(true, 90, snap)
	want := snap.Add(90 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if !RetainUntilForSnapshot(false, 90, snap).IsZero() {
		t.Fatal("expected zero when object lock disabled")
	}
	if !RetainUntilForSnapshot(true, 0, snap).IsZero() {
		t.Fatal("expected zero when ttl is zero")
	}
}

func TestSourceRegistryRetainUntilForSource(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"a": {SourceDefinition: SourceDefinition{ID: "a", Retention: SourceRetention{TTLDays: 7}}},
		},
	}
	snap := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	got := reg.RetainUntilForSource("a", true, snap)
	if got.IsZero() {
		t.Fatal("expected retain until")
	}
	if !reg.RetainUntilForSource("missing", true, snap).IsZero() {
		t.Fatal("unknown source should return zero")
	}
	if !reg.RetainUntilForSource("a", false, snap).IsZero() {
		t.Fatal("global off should return zero")
	}
}