package config

import (
	"fmt"
	"path"
	"strings"
)

// PreBackupConfig is optional Tier 2 dump-before-backup on sftp sources (Phase H).
type PreBackupConfig struct {
	Engine       string            `yaml:"engine,omitempty"`
	Command      []string          `yaml:"command,omitempty"`
	EnvRefs      map[string]string `yaml:"env_refs,omitempty"`
	DatabaseRef  string            `yaml:"database_ref,omitempty"`
	PasswordRef  string            `yaml:"password_ref,omitempty"`
	UserRef      string            `yaml:"user_ref,omitempty"`
	HostRef      string            `yaml:"host_ref,omitempty"`
	Output       PreBackupOutput   `yaml:"output,omitempty"`
}

// PreBackupOutput describes where the remote dump file is written.
type PreBackupOutput struct {
	RelativePath string `yaml:"relative_path,omitempty"`
	CleanupAfter bool   `yaml:"cleanup_after,omitempty"`
}

// ResolvedPreBackup holds material ready for SSH exec (secrets resolved at load).
type ResolvedPreBackup struct {
	Engine         string
	Command        []string
	Env            map[string]string
	Database       string
	Password       string
	PGUser         string
	PGHost         string
	CleanupAfter   bool
	OutputRelative string // if set, overrides schedule relative_path for dump output
}

func validatePreBackup(prefix string, src *SourceDefinition) error {
	if src.PreBackup == nil {
		return nil
	}
	if strings.ToLower(src.Type) != "sftp" {
		return fmt.Errorf("%s (%s): pre_backup is only supported on type sftp", prefix, src.ID)
	}
	pb := src.PreBackup
	engine := strings.ToLower(strings.TrimSpace(pb.Engine))
	hasEngine := engine != ""
	hasCommand := len(pb.Command) > 0
	if hasEngine && hasCommand {
		return fmt.Errorf("%s (%s): pre_backup: set either engine or command, not both", prefix, src.ID)
	}
	if !hasEngine && !hasCommand {
		return fmt.Errorf("%s (%s): pre_backup: engine or command is required", prefix, src.ID)
	}
	switch engine {
	case "", "postgres":
		if engine == "postgres" {
			if strings.TrimSpace(pb.DatabaseRef) == "" {
				return fmt.Errorf("%s (%s): pre_backup.database_ref is required for engine postgres", prefix, src.ID)
			}
			if strings.TrimSpace(pb.PasswordRef) == "" {
				return fmt.Errorf("%s (%s): pre_backup.password_ref is required for engine postgres", prefix, src.ID)
			}
		}
	default:
		return fmt.Errorf("%s (%s): pre_backup: unsupported engine %q", prefix, src.ID, pb.Engine)
	}
	if hasCommand {
		for i, arg := range pb.Command {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("%s (%s): pre_backup.command[%d] is empty", prefix, src.ID, i)
			}
		}
	}
	return nil
}

func resolvePreBackupEnv(prefix string, src *SourceDefinition) (*ResolvedPreBackup, error) {
	if src.PreBackup == nil {
		return nil, nil
	}
	pb := src.PreBackup
	out := &ResolvedPreBackup{
		Engine:         strings.ToLower(strings.TrimSpace(pb.Engine)),
		Command:        append([]string(nil), pb.Command...),
		Env:            make(map[string]string),
		CleanupAfter:   pb.Output.CleanupAfter,
		OutputRelative: NormalizeRelativePath(pb.Output.RelativePath),
	}

	if out.Engine == "postgres" {
		db, err := resolveEnvRef(pb.DatabaseRef, prefix+".pre_backup.database_ref")
		if err != nil {
			return nil, err
		}
		pw, err := resolveEnvRef(pb.PasswordRef, prefix+".pre_backup.password_ref")
		if err != nil {
			return nil, err
		}
		out.Database = db
		out.Password = pw
		if ref := strings.TrimSpace(pb.UserRef); ref != "" {
			u, err := resolveEnvRef(ref, prefix+".pre_backup.user_ref")
			if err != nil {
				return nil, err
			}
			out.PGUser = u
		}
		if ref := strings.TrimSpace(pb.HostRef); ref != "" {
			h, err := resolveEnvRef(ref, prefix+".pre_backup.host_ref")
			if err != nil {
				return nil, err
			}
			out.PGHost = h
		}
	}

	for k, ref := range pb.EnvRefs {
		k = strings.TrimSpace(k)
		if k == "" {
			return nil, fmt.Errorf("%s (%s): pre_backup.env_refs contains empty key", prefix, src.ID)
		}
		v, err := resolveEnvRef(ref, fmt.Sprintf("%s.pre_backup.env_refs.%s", prefix, k))
		if err != nil {
			return nil, err
		}
		out.Env[k] = v
	}
	return out, nil
}

// HasPreBackup reports whether the source runs a dump step before file backup.
func (s ResolvedSource) HasPreBackup() bool {
	return s.ResolvedPreBackup != nil
}

// DumpRelativePath picks the path under remote_root for the dump artifact.
func (s ResolvedSource) DumpRelativePath(scheduleRelative string) string {
	if s.ResolvedPreBackup != nil && s.ResolvedPreBackup.OutputRelative != "" {
		return s.ResolvedPreBackup.OutputRelative
	}
	return NormalizeRelativePath(scheduleRelative)
}

// RemoteDumpPath is the absolute POSIX path on the SSH host.
func (s ResolvedSource) RemoteDumpPath(relativePath string) string {
	root := strings.TrimRight(s.RemoteRoot, "/")
	rel := NormalizeRelativePath(relativePath)
	return path.Join(root, rel)
}

// BuildPreBackupExec returns argv and env for SSH session (no shell).
func (s ResolvedSource) BuildPreBackupExec(dumpRelative string) ([]string, map[string]string, error) {
	if s.ResolvedPreBackup == nil {
		return nil, nil, fmt.Errorf("source %q has no pre_backup", s.ID)
	}
	pb := s.ResolvedPreBackup
	remoteOut := s.RemoteDumpPath(dumpRelative)

	if len(pb.Command) > 0 {
		env := make(map[string]string, len(pb.Env))
		for k, v := range pb.Env {
			env[k] = v
		}
		return append([]string(nil), pb.Command...), env, nil
	}

	switch pb.Engine {
	case "postgres":
		env := map[string]string{"PGPASSWORD": pb.Password}
		if pb.PGUser != "" {
			env["PGUSER"] = pb.PGUser
		}
		if pb.PGHost != "" {
			env["PGHOST"] = pb.PGHost
		}
		for k, v := range pb.Env {
			env[k] = v
		}
		argv := []string{"pg_dump", "-Fc", "-f", remoteOut, pb.Database}
		return argv, env, nil
	default:
		return nil, nil, fmt.Errorf("source %q: unsupported pre_backup engine %q", s.ID, pb.Engine)
	}
}