package config

import (
	"testing"
	"time"
)

func TestDumpFreshnessMaxAge_match(t *testing.T) {
	src := ResolvedSource{
		SourceDefinition: SourceDefinition{
			DumpFreshness: &DumpFreshnessConfig{
				MaxAgeHours:   26,
				RelativePaths: []string{"cosmosbdr/nightly.dump"},
			},
		},
	}
	d, ok := src.DumpFreshnessMaxAge("cosmosbdr/nightly.dump")
	if !ok || d != 26*time.Hour {
		t.Fatalf("got %v ok=%v", d, ok)
	}
	if _, ok := src.DumpFreshnessMaxAge("other.txt"); ok {
		t.Fatal("should not match other path")
	}
}

func TestCheckDumpArtifactFreshness_stale(t *testing.T) {
	old := time.Now().UTC().Add(-48 * time.Hour)
	err := CheckDumpArtifactFreshness(26*time.Hour, old, "cosmosbdr/nightly.dump")
	if err == nil {
		t.Fatal("expected stale error")
	}
}