package activities

import (
	"context"
	"fmt"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/sourceread"
)

func (ja *JobActivities) openSourceReader(ctx context.Context, sourceID, itemPath string) (sourceread.SourceReader, error) {
	sourceID = strings.TrimSpace(sourceID)
	if ja.Registry == nil {
		if sourceID != "" {
			return nil, fmt.Errorf("source registry is not available in worker")
		}
		return sourceread.NewLocalFileReader(itemPath), nil
	}
	return sourceread.Open(ctx, ja.Registry, sourceID, itemPath)
}

func (ja *JobActivities) hashJobItem(ctx context.Context, itemPath, backupKey, sourceID string) (string, error) {
	if sourceID == "" && backupKey != "" {
		if sid, _, err := config.ParseBackupKey(backupKey); err == nil {
			sourceID = sid
		}
	}
	reader, err := ja.openSourceReader(ctx, sourceID, itemPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return reader.ContentSHA256(ctx)
}