package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSourceRegistry_LocalSource(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
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

	reg, err := LoadSourceRegistry(&Config{
		SourcesConfigPath: sources,
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

func TestLoadSourceRegistry_SFTPResolveEnvRefs(t *testing.T) {
	t.Setenv("TEST_SFTP_HOST", "10.0.0.5")
	t.Setenv("TEST_SFTP_USER", "backup")
	t.Setenv("TEST_SFTP_ROOT", "/data")
	t.Setenv("TEST_SFTP_FP", "SHA256:abc")
	t.Setenv("TEST_SFTP_KEY", "-----BEGIN OPENSSH PRIVATE KEY-----\nMC4w\n-----END OPENSSH PRIVATE KEY-----\n")

	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sources, []byte(`version: 1
sources:
  - id: remote
    type: sftp
    host: env:TEST_SFTP_HOST
    user: env:TEST_SFTP_USER
    remote_root: env:TEST_SFTP_ROOT
    ssh:
      secret_ref: TEST_SFTP_KEY
      host_key_fingerprint: env:TEST_SFTP_FP
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadSourceRegistry(&Config{
		SourcesConfigPath: sources,
	})
	if err != nil {
		t.Fatalf("LoadSourceRegistry: %v", err)
	}
	s, ok := reg.Sources["remote"]
	if !ok {
		t.Fatal("missing source")
	}
	if s.Host != "10.0.0.5" || s.User != "backup" || s.RemoteRoot != "/data" {
		t.Fatalf("resolved fields: host=%q user=%q root=%q", s.Host, s.User, s.RemoteRoot)
	}
	if s.SSH.HostKeyFingerprint != "SHA256:abc" {
		t.Fatalf("ssh fingerprint: got %q", s.SSH.HostKeyFingerprint)
	}
}

func TestLoadSourceRegistry_SchedulesUnderSource(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sources, []byte(`version: 1
sources:
  - id: local-host-data
    type: local
    container_mount: /host-data
    schedules:
      - id: nightly-config
        relative_path: config/app.yml
        schedule: "0 2 * * *"
      - id: weekly-tree
        relative_path: data
        schedule: "0 3 * * 0"
        enabled: false
        directory:
          selection: all
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadSourceRegistry(&Config{SourcesConfigPath: sources})
	if err != nil {
		t.Fatalf("LoadSourceRegistry: %v", err)
	}
	enabled := reg.ListEnabledBackupSchedules()
	if len(enabled) != 1 {
		t.Fatalf("enabled schedules: got %d want 1", len(enabled))
	}
	if enabled[0].ID != "nightly-config" || enabled[0].SourceID != "local-host-data" {
		t.Fatalf("schedule: %+v", enabled[0])
	}
}

func TestLoadSourceRegistry_DuplicateScheduleID(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sources, []byte(`version: 1
sources:
  - id: a
    type: local
    container_mount: /host-data
    schedules:
      - id: same-id
        relative_path: foo.txt
        schedule: "0 1 * * *"
  - id: b
    type: local
    container_mount: /other
    schedules:
      - id: same-id
        relative_path: bar.txt
        schedule: "0 2 * * *"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSourceRegistry(&Config{SourcesConfigPath: sources})
	if err == nil {
		t.Fatal("expected duplicate schedule id error")
	}
}

func TestLoadSourceRegistry_SFTPMissingSecret(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources.yaml")
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

	_, err := LoadSourceRegistry(&Config{
		SourcesConfigPath: sources,
	})
	if err == nil {
		t.Fatal("expected error for missing SSH secret env")
	}
}