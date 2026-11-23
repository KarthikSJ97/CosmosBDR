package config

import "testing"

func TestBuildPreBackupExec_postgres(t *testing.T) {
	src := ResolvedSource{
		SourceDefinition: SourceDefinition{
			ID:         "db",
			Type:       "sftp",
			RemoteRoot: "/var/backups",
		},
		ResolvedPreBackup: &ResolvedPreBackup{
			Engine:   "postgres",
			Database: "appdb",
			Password: "secret",
			PGUser:   "postgres",
		},
	}
	argv, env, err := src.BuildPreBackupExec("cosmosbdr/nightly.dump")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 5 || argv[0] != "pg_dump" || argv[3] != "/var/backups/cosmosbdr/nightly.dump" {
		t.Fatalf("argv: %v", argv)
	}
	if env["PGPASSWORD"] != "secret" || env["PGUSER"] != "postgres" {
		t.Fatalf("env: %v", env)
	}
}

func TestValidatePreBackup_rejectsLocal(t *testing.T) {
	src := SourceDefinition{
		ID:   "x",
		Type: "local",
		PreBackup: &PreBackupConfig{
			Engine:      "postgres",
			DatabaseRef: "DB",
			PasswordRef: "PW",
		},
	}
	if err := validatePreBackup("sources[0]", &src); err == nil {
		t.Fatal("expected error for local + pre_backup")
	}
}