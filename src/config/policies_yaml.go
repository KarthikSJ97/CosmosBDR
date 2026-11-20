package config

// BackupPoliciesFile is the on-disk shape of backup-policies.yaml.
type BackupPoliciesFile struct {
	Version  int            `yaml:"version"`
	Policies []BackupPolicy `yaml:"policies"`
}

// BackupPolicy describes what to back up and when (schedule used in Phase E).
type BackupPolicy struct {
	ID        string `yaml:"id"`
	SourceID  string `yaml:"source_id"`
	Path      string `yaml:"path"`
	Kind      string `yaml:"kind"`
	Schedule  string `yaml:"schedule"`
	Enabled   bool   `yaml:"enabled"`
}