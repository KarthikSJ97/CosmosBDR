package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// SourceRegistry is the immutable runtime view of configured backup sources.
type SourceRegistry struct {
	Sources           map[string]ResolvedSource
	defaultLocalMount string
}

// DefaultLocalMount is the container path used for v1 restore of remote-backed files.
func (r *SourceRegistry) DefaultLocalMount() string {
	return r.defaultLocalMount
}

func (r *SourceRegistry) indexDefaultLocalMount() {
	for _, s := range r.Sources {
		if strings.ToLower(s.Type) == "local" && strings.TrimSpace(s.ContainerMount) != "" {
			r.defaultLocalMount = s.ContainerMount
			return
		}
	}
}

// JobPathLayout is the resolved backup job addressing for workflows and readers.
type JobPathLayout struct {
	SourcePathPrefix string // stored on jobs.source_path; joined with file_name in workflow
	ItemPath         string // full path passed to SourceReader
	BackupKey        string
	SourceType       string
	RemoteIO         bool
}

// ResolveJobPaths maps source_id + relative_path for backup/restore API requests.
func (r *SourceRegistry) ResolveJobPaths(sourceID, relativePath string) (JobPathLayout, error) {
	src, ok := r.GetSource(sourceID)
	if !ok {
		return JobPathLayout{}, fmt.Errorf("unknown source_id %q", sourceID)
	}
	rel := NormalizeRelativePath(relativePath)
	if rel == "" || rel == "." {
		return JobPathLayout{}, fmt.Errorf("relative_path is required")
	}
	backupKey := FormatBackupKey(sourceID, rel)
	layout := JobPathLayout{
		BackupKey:  backupKey,
		SourceType: strings.ToLower(src.Type),
	}

	switch strings.ToLower(src.Type) {
	case "local":
		if src.ContainerMount == "" {
			return JobPathLayout{}, fmt.Errorf("source %q: container_mount is required for type local", sourceID)
		}
		layout.ItemPath = filepath.Join(src.ContainerMount, rel)
		layout.SourcePathPrefix = src.ContainerMount
	case "sftp":
		layout.ItemPath = path.Join(src.RemoteRoot, rel)
		layout.SourcePathPrefix = strings.TrimRight(src.RemoteRoot, "/")
		layout.RemoteIO = true
	default:
		return JobPathLayout{}, fmt.Errorf("source %q: unsupported type %q", sourceID, src.Type)
	}
	return layout, nil
}

// RestoreDestination describes where a restore should write by default (same place as backup source).
type RestoreDestination struct {
	SourceID        string
	DestinationPath string
	RemoteIO        bool
}

// RestoreDestinationForBackupKey resolves the default restore target from backup_key metadata.
func (r *SourceRegistry) RestoreDestinationForBackupKey(backupKey string) (RestoreDestination, error) {
	sourceID, rel, err := ParseBackupKey(backupKey)
	if err != nil {
		return RestoreDestination{}, err
	}
	layout, err := r.ResolveJobPaths(sourceID, rel)
	if err != nil {
		return RestoreDestination{}, err
	}
	return RestoreDestination{
		SourceID:        sourceID,
		DestinationPath: layout.ItemPath,
		RemoteIO:        layout.RemoteIO,
	}, nil
}

// LocalRestorePathForBackupKey maps a backup_key to a path under the default local container mount.
// Used when target_path explicitly selects a local staging path on the CosmosBDR host.
func (r *SourceRegistry) LocalRestorePathForBackupKey(backupKey string) (string, error) {
	_, rel, err := ParseBackupKey(backupKey)
	if err != nil {
		return "", err
	}
	mount := r.defaultLocalMount
	if mount == "" {
		return "", fmt.Errorf("no local container_mount source configured for restore")
	}
	return filepath.Join(mount, rel), nil
}

func (r *SourceRegistry) GetSource(id string) (ResolvedSource, bool) {
	s, ok := r.Sources[id]
	return s, ok
}

// ListEnabledBackupSchedules returns all enabled schedules in stable source/schedule order.
func (r *SourceRegistry) ListEnabledBackupSchedules() []ResolvedBackupSchedule {
	sources := r.ListSources()
	out := make([]ResolvedBackupSchedule, 0)
	for _, src := range sources {
		for _, sch := range src.Schedules {
			if !scheduleEnabled(sch) {
				continue
			}
			out = append(out, ResolvedBackupSchedule{
				SourceID:       src.ID,
				BackupSchedule: sch,
			})
		}
	}
	return out
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

// ResolveReadPath maps source_id + relative path to the object path used by SourceReader.
func (r *SourceRegistry) ResolveReadPath(sourceID, relativePath string) (absolutePath, backupKey string, err error) {
	layout, err := r.ResolveJobPaths(sourceID, relativePath)
	if err != nil {
		return "", "", err
	}
	return layout.ItemPath, layout.BackupKey, nil
}