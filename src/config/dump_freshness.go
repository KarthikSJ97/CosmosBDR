package config

import (
	"fmt"
	"strings"
	"time"
)

// DumpFreshnessConfig rejects backup when a dump artifact's mtime is too old (Phase H.1; Tier 1 + Tier 2).
type DumpFreshnessConfig struct {
	MaxAgeHours   int      `yaml:"max_age_hours"`
	RelativePaths []string `yaml:"relative_paths"`
}

func validateDumpFreshness(prefix string, src *SourceDefinition) error {
	if src.DumpFreshness == nil {
		return nil
	}
	df := src.DumpFreshness
	if df.MaxAgeHours <= 0 {
		return fmt.Errorf("%s (%s): dump_freshness.max_age_hours must be positive", prefix, src.ID)
	}
	if len(df.RelativePaths) == 0 {
		return fmt.Errorf("%s (%s): dump_freshness.relative_paths requires at least one path", prefix, src.ID)
	}
	for i, p := range df.RelativePaths {
		if NormalizeRelativePath(p) == "" || NormalizeRelativePath(p) == "." {
			return fmt.Errorf("%s (%s): dump_freshness.relative_paths[%d] is invalid", prefix, src.ID, i)
		}
	}
	return nil
}

// DumpFreshnessMaxAge returns the max age policy when relativePath matches this source config.
func (s ResolvedSource) DumpFreshnessMaxAge(relativePath string) (time.Duration, bool) {
	if s.DumpFreshness == nil {
		return 0, false
	}
	rel := NormalizeRelativePath(relativePath)
	for _, p := range s.DumpFreshness.RelativePaths {
		if NormalizeRelativePath(p) == rel {
			return time.Duration(s.DumpFreshness.MaxAgeHours) * time.Hour, true
		}
	}
	return 0, false
}

// CheckDumpArtifactFreshness fails when modTime is older than the configured max age.
func CheckDumpArtifactFreshness(maxAge time.Duration, modTime time.Time, relativePath string) error {
	if maxAge <= 0 {
		return nil
	}
	modTime = modTime.UTC()
	age := time.Since(modTime)
	if age > maxAge {
		return fmt.Errorf(
			"dump artifact %q too stale: last modified %s (%s ago), max allowed %s",
			relativePath,
			modTime.Format(time.RFC3339),
			age.Round(time.Minute),
			maxAge,
		)
	}
	return nil
}

// RelativePathForInspect resolves the path segment used for dump_freshness lookup.
func RelativePathForInspect(sourceID, backupKey, itemPath string, registry *SourceRegistry) string {
	if backupKey != "" {
		if _, rel, err := ParseBackupKey(backupKey); err == nil {
			return rel
		}
	}
	if registry != nil && sourceID != "" {
		if src, ok := registry.GetSource(sourceID); ok {
			root := strings.TrimRight(src.RemoteRoot, "/")
			if root != "" && strings.HasPrefix(itemPath, root+"/") {
				return NormalizeRelativePath(strings.TrimPrefix(itemPath, root+"/"))
			}
			if src.ContainerMount != "" {
				mount := strings.TrimRight(src.ContainerMount, "/")
				if strings.HasPrefix(itemPath, mount+"/") {
					return NormalizeRelativePath(strings.TrimPrefix(itemPath, mount+"/"))
				}
			}
		}
	}
	return NormalizeRelativePath(itemPath)
}