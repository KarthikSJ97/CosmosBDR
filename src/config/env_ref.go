package config

import (
	"fmt"
	"os"
	"strconv"
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

// resolveEnvStringFieldOptional treats a missing/empty env value as "" (for optional dial overrides).
func resolveEnvStringFieldOptional(fieldName string, value *string) error {
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
	val := strings.TrimSpace(os.Getenv(envName))
	*value = val
	return nil
}

// resolveEnvIntFromVar reads a non-negative integer from an environment variable name.
func resolveEnvIntFromVar(envVar, fieldName string) (int, error) {
	val, err := resolveEnvRef(envVar, fieldName)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return 0, fmt.Errorf("%s: environment variable %q must be an integer: %w", fieldName, envVar, err)
	}
	if n < 0 || n > 65535 {
		return 0, fmt.Errorf("%s: environment variable %q port out of range: %d", fieldName, envVar, n)
	}
	return n, nil
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
	if err := resolveEnvStringField(prefix+".ssh.host_key_fingerprint", &src.SSH.HostKeyFingerprint); err != nil {
		return err
	}
	if err := resolveEnvStringFieldOptional(prefix+".ssh.dial_host", &src.SSH.DialHost); err != nil {
		return err
	}
	if envName := strings.TrimSpace(src.SSH.DialPortEnv); envName != "" {
		if val := strings.TrimSpace(os.Getenv(envName)); val != "" {
			port, err := resolveEnvIntFromVar(envName, prefix+".ssh.dial_port_env")
			if err != nil {
				return err
			}
			src.SSH.DialPort = port
		}
	}
	return nil
}