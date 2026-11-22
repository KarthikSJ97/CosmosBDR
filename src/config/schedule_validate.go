package config

import (
	"fmt"
	"strings"

	"cosmosbdr/src/dirsel"
)

func scheduleEnabled(s BackupSchedule) bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

func validateCronSchedule(schedule string) error {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if strings.HasPrefix(schedule, "@") {
		return nil
	}
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return fmt.Errorf("schedule must be 5-field cron (Cadence) or @descriptor, got %d field(s) in %q", len(fields), schedule)
	}
	return nil
}

func validScheduleID(id string) bool {
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			continue
		default:
			return false
		}
	}
	return true
}

func validateBackupSchedules(prefix, sourceID string, schedules []BackupSchedule, seenIDs map[string]string) error {
	for j, sch := range schedules {
		sp := fmt.Sprintf("%s.schedules[%d]", prefix, j)
		id := strings.TrimSpace(sch.ID)
		if id == "" {
			return fmt.Errorf("%s: id is required", sp)
		}
		if !validScheduleID(id) {
			return fmt.Errorf("%s: id %q must contain only letters, digits, dash, or underscore", sp, id)
		}
		if prev, dup := seenIDs[id]; dup {
			return fmt.Errorf("%s: duplicate schedule id %q (also used by source %q)", sp, id, prev)
		}
		seenIDs[id] = sourceID

		if err := validateCronSchedule(sch.Schedule); err != nil {
			return fmt.Errorf("%s (%s): %w", sp, id, err)
		}

		rel := strings.TrimSpace(sch.RelativePath)
		if sch.Directory != nil {
			if err := dirsel.ValidateScope(sch.Directory); err != nil {
				return fmt.Errorf("%s (%s): directory: %w", sp, id, err)
			}
			if rel == "" {
				rel = "."
			}
			sch.RelativePath = NormalizeRelativePath(rel)
		} else {
			if rel == "" {
				return fmt.Errorf("%s (%s): relative_path is required for single-file schedules", sp, id)
			}
			sch.RelativePath = NormalizeRelativePath(rel)
			if sch.RelativePath == "" || sch.RelativePath == "." {
				return fmt.Errorf("%s (%s): single-file schedule requires a file relative_path (use a directory block for tree backups)", sp, id)
			}
		}
		schedules[j] = sch
	}
	return nil
}