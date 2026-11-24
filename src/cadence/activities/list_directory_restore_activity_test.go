package activities

import "testing"

func TestPathUnderDirectoryRoot(t *testing.T) {
	if !pathUnderDirectoryRoot("dir-backup-test", "dir-backup-test/baseline.txt") {
		t.Fatal("expected under")
	}
	if pathUnderDirectoryRoot("dir-backup-test", "other/baseline.txt") {
		t.Fatal("expected outside")
	}
}

func TestRelativeToDirectoryRoot(t *testing.T) {
	rel, ok := relativeToDirectoryRoot("dir-backup-test", "dir-backup-test/logs/app.log")
	if !ok || rel != "logs/app.log" {
		t.Fatalf("rel=%q ok=%v", rel, ok)
	}
}