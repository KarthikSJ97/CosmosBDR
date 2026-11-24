package sourceread

import (
	"testing"

	"cosmosbdr/src/config"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"
)

func TestListedFilesFromWindowsPaths(t *testing.T) {
	layout := config.DirectoryLayout{
		SourcePathPrefix: "D:/Karthik/cosmosbdr_data",
		ListRootPath:     "D:/Karthik/cosmosbdr_data/dir-backup-test",
	}
	scope := types.DirectoryScope{Selection: dirsel.SelectionAll}
	m, err := dirsel.NewMatcher(scope)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		`D:\Karthik\cosmosbdr_data\dir-backup-test\baseline.txt`,
		`D:/Karthik/cosmosbdr_data/dir-backup-test/docs/guide.pdf`,
	}
	files := listedFilesFromWindowsPaths(layout, m, paths)
	if len(files) != 2 {
		t.Fatalf("got %d files: %+v", len(files), files)
	}
}