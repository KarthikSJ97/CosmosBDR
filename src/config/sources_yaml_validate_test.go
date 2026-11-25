package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func chdirTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

func TestUnmarshalSourcesFileStrict_UnknownField(t *testing.T) {
	_, err := unmarshalSourcesFileStrict([]byte(`version: 1
sources:
  - id: x
    type: local
    container_moun: /data
`))
	if err == nil || !strings.Contains(err.Error(), "container_moun") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestUnmarshalSourcesFileStrict_UnknownScheduleKey(t *testing.T) {
	_, err := unmarshalSourcesFileStrict([]byte(`version: 1
sources:
  - id: x
    type: local
    container_mount: /data
    schedules:
      - id: job
        relative_path: a.txt
        cron: "0 1 * * *"
`))
	if err == nil || !strings.Contains(err.Error(), "cron") {
		t.Fatalf("expected unknown schedule field error, got %v", err)
	}
}

func TestValidateSourceFieldsForType_LocalRejectsSSH(t *testing.T) {
	src := SourceDefinition{
		ID:   "bad",
		Type: "local",
		SSH:  &SSHConfig{SecretRef: "KEY"},
	}
	if err := validateSourceFieldsForType("sources[0]", &src); err == nil {
		t.Fatal("expected error for ssh on local source")
	}
}

func TestCollectSourcesEnvVarNames(t *testing.T) {
	file, err := unmarshalSourcesFileStrict([]byte(`version: 1
sources:
  - id: r
    type: sftp
    host: env:SFTP_HOST
    user: backup
    remote_root: /data
    ssh:
      secret_ref: SFTP_KEY
      host_key_fingerprint: env:SFTP_FP
    pre_backup:
      command: [echo]
      env_refs:
        PGPASSWORD: SFTP_PG_PASS
`))
	if err != nil {
		t.Fatal(err)
	}
	names := collectSourcesEnvVarNames(file)
	want := map[string]bool{"SFTP_HOST": true, "SFTP_KEY": true, "SFTP_FP": true, "SFTP_PG_PASS": true}
	if len(names) != len(want) {
		t.Fatalf("got %v want keys %v", names, want)
	}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("unexpected ref %q", n)
		}
	}
}

func TestValidateSourcesEnvRefs_DotEnvAndRuntime(t *testing.T) {
	dir := chdirTempDir(t)
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte("NEEDED=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEEDED", "value")
	if err := validateSourcesEnvRefs(".env", []string{"NEEDED"}); err != nil {
		t.Fatalf("validateSourcesEnvRefs: %v", err)
	}
}

func TestValidateSourcesEnvRefs_MissingFromDotEnv(t *testing.T) {
	dir := chdirTempDir(t)
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte("OTHER=x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateSourcesEnvRefs(".env", []string{"MISSING_VAR"})
	if err == nil || !strings.Contains(err.Error(), "MISSING_VAR") {
		t.Fatalf("expected missing var error, got %v", err)
	}
}

func TestProjectSourcesYAML_EnvExampleCoverage(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Skip(err)
	}
	sourcesPath := filepath.Join(root, "sources.yaml")
	examplePath := filepath.Join(root, ".env.example")
	sourcesRaw, err := os.ReadFile(sourcesPath)
	if err != nil {
		t.Fatalf("read sources.yaml: %v", err)
	}
	file, err := unmarshalSourcesFileStrict(sourcesRaw)
	if err != nil {
		t.Fatalf("strict parse sources.yaml: %v", err)
	}
	refs := collectSourcesEnvVarNames(file)
	example, err := godotenvReadKeys(examplePath)
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	for _, name := range refs {
		if _, ok := example[name]; !ok {
			t.Errorf("sources.yaml references %q but .env.example has no definition", name)
		}
	}
}

func godotenvReadKeys(path string) (map[string]struct{}, error) {
	m, err := godotenv.Read(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out, nil
}

func findModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}