package config

// SourcesFile is the on-disk shape of sources.yaml.
type SourcesFile struct {
	Version int              `yaml:"version"`
	Sources []SourceDefinition `yaml:"sources"`
}

// SourceDefinition describes one backup origin.
type SourceDefinition struct {
	ID              string          `yaml:"id"`
	Type            string          `yaml:"type"`
	ContainerMount  string          `yaml:"container_mount,omitempty"`
	Host            string          `yaml:"host,omitempty"`
	Port            int             `yaml:"port,omitempty"`
	User            string          `yaml:"user,omitempty"`
	Domain          string          `yaml:"domain,omitempty"`
	RemoteRoot      string          `yaml:"remote_root,omitempty"`
	SSH             *SSHConfig      `yaml:"ssh,omitempty"`
	Retention       SourceRetention `yaml:"retention,omitempty"`
}

// SSHConfig holds non-secret SSH params; secrets are env refs only.
type SSHConfig struct {
	SecretRef          string `yaml:"secret_ref"`
	KnownHostsPolicy   string `yaml:"known_hosts_policy,omitempty"`
	HostKeyFingerprint string `yaml:"host_key_fingerprint,omitempty"`
	ProxyJump          string `yaml:"proxy_jump,omitempty"`
}

// SourceRetention is TTL configuration at source level.
type SourceRetention struct {
	TTLDays int `yaml:"ttl_days"`
}

// ResolvedSSH holds secret material resolved from the environment (never serialized).
type ResolvedSSH struct {
	PrivateKeyPEM string
}

// ResolvedSource is a validated source ready for runtime use.
type ResolvedSource struct {
	SourceDefinition
	ResolvedSecret string
}