package handlers

import (
	"testing"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
)

func TestBackupWorkflowRequestFromSchedule_File(t *testing.T) {
	reg := &config.SourceRegistry{
		Sources: map[string]config.ResolvedSource{
			"local": {
				SourceDefinition: config.SourceDefinition{
					ID:             "local",
					Type:           "local",
					ContainerMount: "/host-data",
				},
			},
		},
	}
	req, err := BackupWorkflowRequestFromSchedule(reg, config.ResolvedBackupSchedule{
		SourceID: "local",
		BackupSchedule: config.BackupSchedule{
			ID:           "nightly",
			RelativePath: "config/app.yml",
			Schedule:     "0 2 * * *",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.FlowType != string(schema.FlowBackup) {
		t.Fatalf("flow: %q", req.FlowType)
	}
	if req.Payload.ScheduleJobID != "nightly" || req.Payload.BackupKey != "local:config/app.yml" {
		t.Fatalf("payload: %+v", req.Payload)
	}
	if req.SourcePath != "/host-data" {
		t.Fatalf("source path: %q", req.SourcePath)
	}
}

func TestBackupWorkflowRequestFromSchedule_Directory(t *testing.T) {
	reg := &config.SourceRegistry{
		Sources: map[string]config.ResolvedSource{
			"local": {
				SourceDefinition: config.SourceDefinition{
					ID:             "local",
					Type:           "local",
					ContainerMount: "/host-data",
				},
			},
		},
	}

	req, err := BackupWorkflowRequestFromSchedule(reg, config.ResolvedBackupSchedule{
		SourceID: "local",
		BackupSchedule: config.BackupSchedule{
			ID:           "weekly",
			RelativePath: "data",
			Schedule:     "0 3 * * 0",
			Directory: &types.DirectoryScope{
				Selection: "all",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !IsDirectoryBackup(&req) || req.ListRootPath != "/host-data/data" {
		t.Fatalf("directory layout: list=%q dir=%+v", req.ListRootPath, req.Payload.Directory)
	}
}