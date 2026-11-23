package config

import "testing"

func TestResolveJobPaths_SFTP(t *testing.T) {
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
	reg.indexDefaultLocalMount()

	layout, err := reg.ResolveJobPaths("remote", "app/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if layout.ItemPath != "/var/data/app/config.yml" {
		t.Fatalf("item path: %q", layout.ItemPath)
	}
	if !layout.RemoteIO {
		t.Fatal("expected RemoteIO")
	}
	if layout.BackupKey != "remote:app/config.yml" {
		t.Fatalf("backup key: %q", layout.BackupKey)
	}
}

func TestRestoreDestinationForBackupKey_Local(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"local-host-data": {
				SourceDefinition: SourceDefinition{
					ID:             "local-host-data",
					Type:           "local",
					ContainerMount: "/host-data",
				},
			},
		},
	}
	reg.indexDefaultLocalMount()

	dest, err := reg.RestoreDestinationForBackupKey("local-host-data:app/x.dat")
	if err != nil {
		t.Fatal(err)
	}
	if dest.DestinationPath != "/host-data/app/x.dat" {
		t.Fatalf("restore path: %q", dest.DestinationPath)
	}
	if dest.RemoteIO {
		t.Fatal("expected local restore")
	}
}

func TestRestoreDestinationForBackupKey_SFTP(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"win-home-laptop": {
				SourceDefinition: SourceDefinition{
					ID:         "win-home-laptop",
					Type:       "sftp",
					RemoteRoot: "/Users/karthik",
				},
			},
		},
	}
	reg.indexDefaultLocalMount()

	dest, err := reg.RestoreDestinationForBackupKey("win-home-laptop:My Memes.pdf")
	if err != nil {
		t.Fatal(err)
	}
	want := "/Users/karthik/My Memes.pdf"
	if dest.DestinationPath != want {
		t.Fatalf("restore path: %q want %q", dest.DestinationPath, want)
	}
	if !dest.RemoteIO {
		t.Fatal("expected remote restore")
	}
	if dest.SourceID != "win-home-laptop" {
		t.Fatalf("source id: %q", dest.SourceID)
	}
}

func TestListRetentionGCSources_SkipsUnconfigured(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"no-retention": {
				SourceDefinition: SourceDefinition{ID: "no-retention", Type: "local", ContainerMount: "/data"},
			},
			"with-retention": {
				SourceDefinition: SourceDefinition{
					ID:             "with-retention",
					Type:           "local",
					ContainerMount: "/data",
					Retention:      SourceRetention{TTLDays: 14},
				},
			},
			"zero-ttl": {
				SourceDefinition: SourceDefinition{
					ID:             "zero-ttl",
					Type:           "local",
					ContainerMount: "/data",
					Retention:      SourceRetention{TTLDays: 0},
				},
			},
		},
	}
	got := reg.ListRetentionGCSources()
	if len(got) != 1 || got[0].SourceID != "with-retention" || got[0].TTLDays != 14 {
		t.Fatalf("retention GC sources: %+v", got)
	}
}

func TestSourceRetention_GCEnabled(t *testing.T) {
	if (SourceRetention{}).GCEnabled() {
		t.Fatal("empty retention should disable GC")
	}
	if !(SourceRetention{TTLDays: 1}).GCEnabled() {
		t.Fatal("ttl_days > 0 should enable GC")
	}
}

func TestLocalRestorePathForBackupKey_StagingOverride(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"local-host-data": {
				SourceDefinition: SourceDefinition{
					ID:             "local-host-data",
					Type:           "local",
					ContainerMount: "/host-data",
				},
			},
		},
	}
	reg.indexDefaultLocalMount()
	path, err := reg.LocalRestorePathForBackupKey("remote:app/x.dat")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/host-data/app/x.dat" {
		t.Fatalf("staging path: %q", path)
	}
}