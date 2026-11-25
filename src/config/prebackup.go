package config

import (
	"fmt"
	"path"
	"strings"
)

// PreBackupConfig is an optional remote hook on sftp sources: SSH exec once before backup (Phase H).
// All behavior is defined in sources.yaml via command + env_refs (no code-level engine presets).
type PreBackupConfig struct {
	Command []string          `yaml:"command"`
	EnvRefs map[string]string `yaml:"env_refs,omitempty"`
	Output  PreBackupOutput   `yaml:"output,omitempty"`
}

// PreBackupOutput optional remote artifact path and post-success cleanup.
type PreBackupOutput struct {
	RelativePath string `yaml:"relative_path,omitempty"`
	CleanupAfter bool   `yaml:"cleanup_after,omitempty"`
}

// ResolvedPreBackup holds argv/env ready for SSH exec (secrets resolved at load).
type ResolvedPreBackup struct {
	Command        []string
	Env            map[string]string
	CleanupAfter   bool
	OutputRelative string // if set, overrides schedule/API relative_path for single-file backup target
}

func validatePreBackup(prefix string, src *SourceDefinition) error {
	if src.PreBackup == nil {
		return nil
	}
	if strings.ToLower(src.Type) != "sftp" {
		return fmt.Errorf("%s (%s): pre_backup is only supported on type sftp", prefix, src.ID)
	}
	pb := src.PreBackup
	if len(pb.Command) == 0 {
		return fmt.Errorf("%s (%s): pre_backup.command is required", prefix, src.ID)
	}
	for i, arg := range pb.Command {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("%s (%s): pre_backup.command[%d] is empty", prefix, src.ID, i)
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
		Command:        append([]string(nil), pb.Command...),
		Env:            make(map[string]string),
		CleanupAfter:   pb.Output.CleanupAfter,
		OutputRelative: NormalizeRelativePath(pb.Output.RelativePath),
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

// HasPreBackup reports whether the source runs a remote hook before backup jobs.
func (s ResolvedSource) HasPreBackup() bool {
	return s.ResolvedPreBackup != nil
}

// PreBackupOutputRelative resolves the path under remote_root for single-file backup when output is set.
func (s ResolvedSource) PreBackupOutputRelative(scheduleRelative string) string {
	if s.ResolvedPreBackup != nil && s.ResolvedPreBackup.OutputRelative != "" {
		return s.ResolvedPreBackup.OutputRelative
	}
	return NormalizeRelativePath(scheduleRelative)
}

// RemoteOutputPath is the absolute POSIX path on the SSH host for a relative path under remote_root.
func (s ResolvedSource) RemoteOutputPath(relativePath string) string {
	root := strings.TrimRight(s.RemoteRoot, "/")
	rel := NormalizeRelativePath(relativePath)
	return path.Join(root, rel)
}

// BuildPreBackupExec returns argv and env for SSH session (no shell).
func (s ResolvedSource) BuildPreBackupExec() ([]string, map[string]string, error) {
	if s.ResolvedPreBackup == nil {
		return nil, nil, fmt.Errorf("source %q has no pre_backup", s.ID)
	}
	pb := s.ResolvedPreBackup
	if len(pb.Command) == 0 {
		return nil, nil, fmt.Errorf("source %q: pre_backup command is empty", s.ID)
	}
	env := make(map[string]string, len(pb.Env))
	for k, v := range pb.Env {
		env[k] = v
	}
	return append([]string(nil), pb.Command...), env, nil
}