package config

import "testing"

func TestBuildPreBackupExec_command(t *testing.T) {
	src := ResolvedSource{
		SourceDefinition: SourceDefinition{
			ID:         "db",
			Type:       "sftp",
			RemoteRoot: "/var/backups",
		},
		ResolvedPreBackup: &ResolvedPreBackup{
			Command: []string{"bash", "-lc", "echo ok"},
			Env:     map[string]string{"PGPASSWORD": "secret"},
		},
	}
	argv, env, err := src.BuildPreBackupExec()
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 3 || argv[0] != "bash" {
		t.Fatalf("argv: %v", argv)
	}
	if env["PGPASSWORD"] != "secret" {
		t.Fatalf("env: %v", env)
	}
	if src.RemoteOutputPath("db/nightly.dump") != "/var/backups/db/nightly.dump" {
		t.Fatalf("remote output path")
	}
}

func TestValidatePreBackup_requiresCommand(t *testing.T) {
	src := SourceDefinition{
		ID:   "x",
		Type: "sftp",
		PreBackup: &PreBackupConfig{
			Output: PreBackupOutput{RelativePath: "out.dump"},
		},
	}
	if err := validatePreBackup("sources[0]", &src); err == nil {
		t.Fatal("expected error without command")
	}
}

func TestValidatePreBackup_rejectsLocal(t *testing.T) {
	src := SourceDefinition{
		ID:   "x",
		Type: "local",
		PreBackup: &PreBackupConfig{
			Command: []string{"true"},
		},
	}
	if err := validatePreBackup("sources[0]", &src); err == nil {
		t.Fatal("expected error for local + pre_backup")
	}
}