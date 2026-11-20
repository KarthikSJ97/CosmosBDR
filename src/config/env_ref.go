package config

import (
	"fmt"
	"strings"
)

const envRefPrefix = "env:"

// EnvPrefixFromSourceID maps a sources.yaml id to the env var prefix (win-home-laptop → WIN_HOME_LAPTOP).
func EnvPrefixFromSourceID(sourceID string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(sourceID), "-", "_"))
}

// resolveEnvStringField replaces values prefixed with "env:VAR" with os.Getenv("VAR").
func resolveEnvStringField(fieldName string, value *string) error {
	if value == nil {
		return nil
	}
	raw := strings.TrimSpace(*value)
	if raw == "" || !strings.HasPrefix(raw, envRefPrefix) {
		return nil
	}
	envName := strings.TrimSpace(strings.TrimPrefix(raw, envRefPrefix))
	if envName == "" {
		return fmt.Errorf("%s: env ref is empty", fieldName)
	}
	resolved, err := resolveEnvRef(envName, fieldName)
	if err != nil {
		return err
	}
	*value = resolved
	return nil
}

// resolveSFTPSourceEnv fills SFTP connection fields from the environment when configured.
func resolveSFTPSourceEnv(src *SourceDefinition, index int) error {
	prefix := fmt.Sprintf("sources[%d] (%s)", index, src.ID)
	if err := resolveEnvStringField(prefix+".host", &src.Host); err != nil {
		return err
	}
	if err := resolveEnvStringField(prefix+".user", &src.User); err != nil {
		return err
	}
	if err := resolveEnvStringField(prefix+".remote_root", &src.RemoteRoot); err != nil {
		return err
	}
	if src.SSH == nil {
		return nil
	}
	return resolveEnvStringField(prefix+".ssh.host_key_fingerprint", &src.SSH.HostKeyFingerprint)
}