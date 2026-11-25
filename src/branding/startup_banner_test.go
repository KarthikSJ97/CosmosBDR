package branding

import (
	"testing"
)

func TestTaglineNonEmpty(t *testing.T) {
	if Tagline == "" {
		t.Fatal("expected tagline")
	}
}

func TestPrintStartupBanner_NoPanic(t *testing.T) {
	// stdout is usually not a TTY in `go test`; must not panic either way.
	PrintStartupBanner()
}