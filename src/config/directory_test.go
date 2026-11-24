package config

import "testing"

func TestResolveDirectoryLayout_LocalDot(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"local": {
				SourceDefinition: SourceDefinition{
					ID:             "local",
					Type:           "local",
					ContainerMount: "/host-data",
				},
			},
		},
	}
	layout, err := reg.ResolveDirectoryLayout("local", ".")
	if err != nil {
		t.Fatal(err)
	}
	if layout.ListRootPath != "/host-data" {
		t.Fatalf("list root: %q", layout.ListRootPath)
	}
	if layout.DirRelative != "." {
		t.Fatalf("dir rel: %q", layout.DirRelative)
	}
}

func TestResolveDirectoryLayout_SFTPSubdir(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"remote": {
				SourceDefinition: SourceDefinition{
					ID:         "remote",
					Type:       "sftp",
					RemoteRoot: "/var/data",
				},
			},
		},
	}
	layout, err := reg.ResolveDirectoryLayout("remote", "backups")
	if err != nil {
		t.Fatal(err)
	}
	if layout.ListRootPath != "/var/data/backups" {
		t.Fatalf("list root: %q", layout.ListRootPath)
	}
	if !layout.RemoteIO {
		t.Fatal("expected remote IO")
	}
}

func TestRelToSource_RemoteOutsideRoot(t *testing.T) {
	layout := DirectoryLayout{
		SourcePathPrefix: "D:/Karthik/cosmosbdr_data",
		ListRootPath:     "D:/Karthik/cosmosbdr_data/dir-backup-test",
	}
	_, err := RelToSource(layout, ".cache/huggingface/agent.json", true)
	if err == nil {
		t.Fatal("expected error for path outside source root")
	}
}

func TestRelToDir_RemoteOutsideDirectoryRoot(t *testing.T) {
	layout := DirectoryLayout{
		SourcePathPrefix: "D:/Karthik/cosmosbdr_data",
		ListRootPath:     "D:/Karthik/cosmosbdr_data/dir-backup-test",
	}
	_, err := RelToDir(layout, "D:/Karthik/cosmosbdr_data/.cache/x", true)
	if err == nil {
		t.Fatal("expected error for path outside directory list root")
	}
}

func TestRelToDir_Local(t *testing.T) {
	layout := DirectoryLayout{
		SourcePathPrefix: "/host-data",
		ListRootPath:     "/host-data/conf",
		DirRelative:      "conf",
	}
	rel, err := RelToDir(layout, "/host-data/conf/app.json", false)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "app.json" {
		t.Fatalf("rel to dir: %q", rel)
	}
}