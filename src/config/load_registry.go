package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSourceRegistry reads, validates, and resolves sources from sources.yaml.
func LoadSourceRegistry(cfg *Config) (*SourceRegistry, error) {
	sourcesPath := cfg.SourcesConfigPath

	sourcesRaw, err := os.ReadFile(sourcesPath)
	if err != nil {
		return nil, fmt.Errorf("read sources config %q: %w", sourcesPath, err)
	}

	var sourcesFile SourcesFile
	if err := yaml.Unmarshal(sourcesRaw, &sourcesFile); err != nil {
		return nil, fmt.Errorf("parse sources config: %w", err)
	}

	if sourcesFile.Version != 1 {
		return nil, fmt.Errorf("sources.yaml: unsupported version %d (want 1)", sourcesFile.Version)
	}
	if len(sourcesFile.Sources) == 0 {
		return nil, fmt.Errorf("sources.yaml: at least one source is required")
	}

	reg := &SourceRegistry{
		Sources: make(map[string]ResolvedSource),
	}

	seenSource := make(map[string]struct{})
	seenScheduleIDs := make(map[string]string)
	for i, src := range sourcesFile.Sources {
		if err := validateSource(&src, i); err != nil {
			return nil, err
		}
		prefix := fmt.Sprintf("sources[%d]", i)
		if err := validatePreBackup(prefix, &src); err != nil {
			return nil, err
		}
		if err := validateDumpFreshness(prefix, &src); err != nil {
			return nil, err
		}
		prefix = fmt.Sprintf("sources[%d] (%s)", i, src.ID)
		if err := validateBackupSchedules(prefix, src.ID, src.Schedules, seenScheduleIDs); err != nil {
			return nil, err
		}
		if strings.ToLower(src.Type) == "sftp" {
			if err := resolveSFTPSourceEnv(&src, i); err != nil {
				return nil, err
			}
			if err := validateSource(&src, i); err != nil {
				return nil, err
			}
		}
		if _, dup := seenSource[src.ID]; dup {
			return nil, fmt.Errorf("sources.yaml: duplicate source id %q", src.ID)
		}
		seenSource[src.ID] = struct{}{}

		resolved := ResolvedSource{SourceDefinition: src}
		if strings.ToLower(src.Type) == "sftp" {
			secret, err := resolveEnvRef(src.SSH.SecretRef, fmt.Sprintf("sources[%d].ssh.secret_ref", i))
			if err != nil {
				return nil, err
			}
			resolved.ResolvedSecret = secret
			pre, err := resolvePreBackupEnv(prefix, &src)
			if err != nil {
				return nil, err
			}
			resolved.ResolvedPreBackup = pre
		}
		reg.Sources[src.ID] = resolved
	}

	reg.indexDefaultLocalMount()

	return reg, nil
}

func validateSource(src *SourceDefinition, index int) error {
	prefix := fmt.Sprintf("sources[%d]", index)
	if strings.TrimSpace(src.ID) == "" {
		return fmt.Errorf("%s: id is required", prefix)
	}
	typ := strings.ToLower(strings.TrimSpace(src.Type))
	if typ == "" {
		return fmt.Errorf("%s (%s): type is required", prefix, src.ID)
	}
	src.Type = typ

	switch typ {
	case "local":
		if strings.TrimSpace(src.ContainerMount) == "" {
			return fmt.Errorf("%s (%s): container_mount is required for type local", prefix, src.ID)
		}
	case "sftp":
		if strings.TrimSpace(src.Host) == "" {
			return fmt.Errorf("%s (%s): host is required for type sftp", prefix, src.ID)
		}
		if src.Port == 0 {
			src.Port = 22
		}
		if strings.TrimSpace(src.User) == "" {
			return fmt.Errorf("%s (%s): user is required for type sftp", prefix, src.ID)
		}
		if strings.TrimSpace(src.RemoteRoot) == "" {
			return fmt.Errorf("%s (%s): remote_root is required for type sftp", prefix, src.ID)
		}
		if src.SSH == nil {
			return fmt.Errorf("%s (%s): ssh block is required for type sftp", prefix, src.ID)
		}
		if strings.TrimSpace(src.SSH.SecretRef) == "" {
			return fmt.Errorf("%s (%s): ssh.secret_ref is required", prefix, src.ID)
		}
	default:
		return fmt.Errorf("%s (%s): unsupported type %q", prefix, src.ID, typ)
	}

	if src.Retention.TTLDays < 0 {
		return fmt.Errorf("%s (%s): retention.ttl_days cannot be negative", prefix, src.ID)
	}
	return nil
}

func resolveEnvRef(ref, field string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("%s: secret ref is empty", field)
	}
	val := os.Getenv(ref)
	if val == "" {
		return "", fmt.Errorf("%s: environment variable %q is not set or empty", field, ref)
	}
	// Allow single-line .env PEM values with literal \n sequences.
	val = strings.ReplaceAll(val, `\n`, "\n")
	return val, nil
}