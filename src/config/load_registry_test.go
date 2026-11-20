package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSourceRegistry_LocalSource(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
	policies := filepath.Join(dir, "policies.yaml")
	if err := os.WriteFile(sources, []byte(`version: 1
sources:
  - id: local-host-data
    type: local
    container_mount: /host-data
    retention:
      ttl_days: 30
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policies, []byte(`version: 1
policies:
  - id: p1
    source_id: local-host-data
    path: foo.txt
    kind: file
    schedule: "0 1 * * *"
    enabled: false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadSourceRegistry(&Config{
		SourcesConfigPath:        sources,
		BackupPoliciesConfigPath: policies,
	})
	if err != nil {
		t.Fatalf("LoadSourceRegistry: %v", err)
	}
	abs, key, err := reg.ResolveReadPath("local-host-data", "dir/foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if abs != "/host-data/dir/foo.txt" {
		t.Fatalf("abs path: got %q", abs)
	}
	if key != "local-host-data:dir/foo.txt" {
		t.Fatalf("backup key: got %q", key)
	}
}

func TestLoadSourceRegistry_SFTPMissingSecret(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
	policies := filepath.Join(dir, "policies.yaml")
	if err := os.WriteFile(sources, []byte(`version: 1
sources:
  - id: remote
    type: sftp
    host: example.com
    user: backup
    remote_root: /data
    ssh:
      secret_ref: MISSING_SSH_KEY
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policies, []byte(`version: 1
policies: []
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSourceRegistry(&Config{
		SourcesConfigPath:        sources,
		BackupPoliciesConfigPath: policies,
	})
	if err == nil {
		t.Fatal("expected error for missing SSH secret env")
	}
}