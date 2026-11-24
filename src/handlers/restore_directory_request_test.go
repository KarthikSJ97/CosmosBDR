package handlers

import (
	"testing"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
)

func TestIsDirectoryRestore(t *testing.T) {
	dir := &types.DirectoryScope{Selection: "all"}
	req := &types.BackupWorkflowRequest{
		FlowType: string(schema.FlowRestore),
		Payload: types.BackupJobPayload{
			SourceID:             "win-home-laptop",
			RelativePath:         "dir-backup-test",
			SourceDirectoryJobID: "parent-uuid",
		},
	}
	if !IsDirectoryRestore(req) {
		t.Fatal("expected directory restore via parent job id")
	}
	req.Payload.SourceDirectoryJobID = ""
	req.Payload.Directory = dir
	if !IsDirectoryRestore(req) {
		t.Fatal("expected directory restore via directory scope")
	}
	file := &types.BackupWorkflowRequest{
		FlowType: string(schema.FlowRestore),
		Payload: types.BackupJobPayload{
			SourceID:     "win-home-laptop",
			RelativePath: "dir-backup-test/baseline.txt",
		},
	}
	if IsDirectoryRestore(file) {
		t.Fatal("single-file restore should not be directory restore")
	}
}

func TestResolveDirectoryRestoreRequest(t *testing.T) {
	reg := &config.SourceRegistry{
		Sources: map[string]config.ResolvedSource{
			"remote": {
				SourceDefinition: config.SourceDefinition{
					ID:         "remote",
					Type:       "sftp",
					RemoteRoot: "D:/data",
				},
			},
		},
	}
	req := types.BackupWorkflowRequest{
		FlowType: string(schema.FlowRestore),
		Payload: types.BackupJobPayload{
			SourceID:             "remote",
			RelativePath:         "backups",
			SourceDirectoryJobID: "aed6289c-ce59-4f6e-9d2a-d5f703dd1310",
		},
	}
	if err := ResolveDirectoryRestoreRequest(reg, &req); err != nil {
		t.Fatal(err)
	}
	if req.SourcePath != "D:/data" || !req.Payload.RemoteIO {
		t.Fatalf("resolved: path=%q remote=%v", req.SourcePath, req.Payload.RemoteIO)
	}
}