package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SourceRegistry is the immutable runtime view of sources and policies.
type SourceRegistry struct {
	Sources  map[string]ResolvedSource
	Policies map[string]BackupPolicy
}

func (r *SourceRegistry) GetSource(id string) (ResolvedSource, bool) {
	s, ok := r.Sources[id]
	return s, ok
}

func (r *SourceRegistry) GetPolicy(id string) (BackupPolicy, bool) {
	p, ok := r.Policies[id]
	return p, ok
}

// ListSources returns sources in stable ID order.
func (r *SourceRegistry) ListSources() []ResolvedSource {
	ids := make([]string, 0, len(r.Sources))
	for id := range r.Sources {
		ids = append(ids, id)
	}
	// simple sort
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	out := make([]ResolvedSource, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.Sources[id])
	}
	return out
}

// ListPolicies returns all policies.
func (r *SourceRegistry) ListPolicies() []BackupPolicy {
	out := make([]BackupPolicy, 0, len(r.Policies))
	for _, p := range r.Policies {
		out = append(out, p)
	}
	return out
}

// ResolveReadPath maps source_id + relative path to the absolute path used by local readers.
func (r *SourceRegistry) ResolveReadPath(sourceID, relativePath string) (absolutePath, backupKey string, err error) {
	src, ok := r.GetSource(sourceID)
	if !ok {
		return "", "", fmt.Errorf("unknown source_id %q", sourceID)
	}
	rel := NormalizeRelativePath(relativePath)
	if rel == "" || rel == "." {
		return "", "", fmt.Errorf("relative_path is required")
	}
	backupKey = FormatBackupKey(sourceID, rel)

	switch strings.ToLower(src.Type) {
	case "local":
		if src.ContainerMount == "" {
			return "", "", fmt.Errorf("source %q: container_mount is required for type local", sourceID)
		}
		absolutePath = filepath.Join(src.ContainerMount, rel)
		return absolutePath, backupKey, nil
	case "sftp":
		return "", backupKey, fmt.Errorf("source %q: SFTP read path is not implemented yet (Phase B)", sourceID)
	default:
		return "", "", fmt.Errorf("source %q: unsupported type %q", sourceID, src.Type)
	}
}