package config

import "cosmosbdr/src/types"

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
	PreBackup       *PreBackupConfig  `yaml:"pre_backup,omitempty"`
	DumpFreshness   *DumpFreshnessConfig `yaml:"dump_freshness,omitempty"`
	Retention       SourceRetention   `yaml:"retention,omitempty"`
	Schedules       []BackupSchedule `yaml:"schedules,omitempty"`
}

// BackupSchedule is one Cadence cron target under a source (Phase E).
type BackupSchedule struct {
	ID           string                `yaml:"id"`
	RelativePath string                `yaml:"relative_path"`
	Schedule     string                `yaml:"schedule"`
	Enabled      *bool                 `yaml:"enabled,omitempty"` // default true when omitted
	Directory    *types.DirectoryScope `yaml:"directory,omitempty"`
}

// ResolvedBackupSchedule is a validated schedule ready for cron registration.
type ResolvedBackupSchedule struct {
	SourceID string
	BackupSchedule
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

// GCEnabled reports whether retention GC should run for this source (ttl_days > 0).
// Omitted retention or ttl_days: 0 disables expiry for that source.
func (r SourceRetention) GCEnabled() bool {
	return r.TTLDays > 0
}

// ResolvedSSH holds secret material resolved from the environment (never serialized).
type ResolvedSSH struct {
	PrivateKeyPEM string
}

// ResolvedSource is a validated source ready for runtime use.
type ResolvedSource struct {
	SourceDefinition
	ResolvedSecret    string
	ResolvedPreBackup *ResolvedPreBackup
}