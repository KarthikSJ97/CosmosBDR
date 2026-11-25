package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const defaultDotEnvPath = ".env"

// unmarshalSourcesFileStrict parses sources.yaml and rejects unknown keys at any nesting level.
func unmarshalSourcesFileStrict(raw []byte) (SourcesFile, error) {
	var file SourcesFile
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return SourcesFile{}, fmt.Errorf("parse sources config: %w", err)
	}
	return file, nil
}

// collectSourcesEnvVarNames returns unique env var names referenced by sources.yaml (env: fields and *_ref).
func collectSourcesEnvVarNames(file SourcesFile) []string {
	seen := make(map[string]struct{})
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	addEnvField := func(value string) {
		if name, ok := envRefName(value); ok {
			add(name)
		}
	}

	for _, src := range file.Sources {
		addEnvField(src.Host)
		addEnvField(src.User)
		addEnvField(src.Domain)
		addEnvField(src.RemoteRoot)
		if src.SSH != nil {
			add(src.SSH.SecretRef)
			add(src.SSH.PasswordRef)
			addEnvField(src.SSH.HostKeyFingerprint)
			addEnvField(src.SSH.ProxyJump)
		}
		if src.PreBackup != nil {
			for _, ref := range src.PreBackup.EnvRefs {
				add(ref)
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func envRefName(value string) (string, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" || !strings.HasPrefix(raw, envRefPrefix) {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimPrefix(raw, envRefPrefix))
	if name == "" {
		return "", false
	}
	return name, true
}

// validateSourcesEnvRefs ensures env vars referenced in sources.yaml are defined in dotEnvPath (when present) and non-empty at runtime.
func validateSourcesEnvRefs(dotEnvPath string, varNames []string) error {
	if len(varNames) == 0 {
		return nil
	}
	var dotEnv map[string]string
	if _, err := os.Stat(dotEnvPath); err == nil {
		m, err := godotenv.Read(dotEnvPath)
		if err != nil {
			return fmt.Errorf("read %q: %w", dotEnvPath, err)
		}
		dotEnv = m
	}
	for _, name := range varNames {
		if dotEnv != nil {
			val, ok := dotEnv[name]
			if !ok {
				return fmt.Errorf("sources.yaml requires environment variable %q but it is not defined in %q", name, dotEnvPath)
			}
			if strings.TrimSpace(val) == "" {
				return fmt.Errorf("sources.yaml requires environment variable %q but %q defines it as empty", name, dotEnvPath)
			}
		}
		if _, err := resolveEnvRef(name, "sources.yaml"); err != nil {
			return err
		}
	}
	return nil
}

func validateSourceFieldsForType(prefix string, src *SourceDefinition) error {
	typ := strings.ToLower(strings.TrimSpace(src.Type))
	switch typ {
	case "local":
		var bad []string
		if strings.TrimSpace(src.Host) != "" {
			bad = append(bad, "host")
		}
		if src.Port != 0 && src.Port != 22 {
			bad = append(bad, "port")
		}
		if strings.TrimSpace(src.User) != "" {
			bad = append(bad, "user")
		}
		if strings.TrimSpace(src.Domain) != "" {
			bad = append(bad, "domain")
		}
		if strings.TrimSpace(src.RemoteRoot) != "" {
			bad = append(bad, "remote_root")
		}
		if src.SSH != nil {
			bad = append(bad, "ssh")
		}
		if src.PreBackup != nil {
			bad = append(bad, "pre_backup")
		}
		if len(bad) > 0 {
			return fmt.Errorf("%s (%s): field(s) %s are not valid for type local (use type sftp)", prefix, src.ID, strings.Join(bad, ", "))
		}
	case "sftp":
		if strings.TrimSpace(src.ContainerMount) != "" {
			return fmt.Errorf("%s (%s): container_mount is only valid for type local", prefix, src.ID)
		}
		if src.SSH != nil {
			pol := strings.ToLower(strings.TrimSpace(src.SSH.KnownHostsPolicy))
			if pol != "" && pol != "pinned" {
				return fmt.Errorf("%s (%s): ssh.known_hosts_policy must be %q or omitted (got %q)", prefix, src.ID, "pinned", src.SSH.KnownHostsPolicy)
			}
		}
	}
	return nil
}