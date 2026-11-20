package config

import "testing"

func TestFormatParseBackupKey(t *testing.T) {
	key := FormatBackupKey("local-host-data", "foo/bar.txt")
	if key != "local-host-data:foo/bar.txt" {
		t.Fatalf("format: %q", key)
	}
	sid, rel, err := ParseBackupKey(key)
	if err != nil || sid != "local-host-data" || rel != "foo/bar.txt" {
		t.Fatalf("parse: %s %s %v", sid, rel, err)
	}
}