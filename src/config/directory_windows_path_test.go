package config

import "testing"

func TestRelToSource_WindowsDrivePaths(t *testing.T) {
	layout := DirectoryLayout{
		SourcePathPrefix: "D:/Karthik/cosmosbdr_data",
		ListRootPath:     "D:/Karthik/cosmosbdr_data/dir-backup-test",
	}
	abs := "D:/Karthik/cosmosbdr_data/dir-backup-test/baseline.txt"
	rel, err := RelToSource(layout, abs, true)
	if err != nil {
		t.Fatalf("RelToSource: %v", err)
	}
	if rel != "dir-backup-test/baseline.txt" {
		t.Fatalf("rel: %q", rel)
	}
	relDir, err := RelToDir(layout, abs, true)
	if err != nil {
		t.Fatalf("RelToDir: %v", err)
	}
	if relDir != "baseline.txt" {
		t.Fatalf("relDir: %q", relDir)
	}

	lower := `d:\Karthik\cosmosbdr_data\dir-backup-test\baseline.txt`
	rel2, err := RelToSource(layout, lower, true)
	if err != nil {
		t.Fatalf("RelToSource mixed case: %v", err)
	}
	if rel2 != "dir-backup-test/baseline.txt" {
		t.Fatalf("rel mixed: %q", rel2)
	}

	ext := `\\?\D:\Karthik\cosmosbdr_data\dir-backup-test\baseline.txt`
	relDir, err = RelToDir(layout, ext, true)
	if err != nil {
		t.Fatalf("RelToDir extended: %v", err)
	}
	if relDir != "baseline.txt" {
		t.Fatalf("relDir extended: %q", relDir)
	}
}