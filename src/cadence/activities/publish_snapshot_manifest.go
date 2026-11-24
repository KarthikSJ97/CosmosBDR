package activities

import (
	"bytes"
	"context"
	"fmt"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/manifest"
	"cosmosbdr/src/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// publishSnapshotManifest writes snapshots/<time>__<itemID>/manifest.json for manual / DR restore.
// fileChecksum may be empty when the job item already carries a checksum (e.g. dedup skip).
func (ja *JobActivities) publishSnapshotManifest(ctx context.Context, item *schema.JobItem, filePrefix, fileChecksum string) (string, error) {
	if filePrefix == "" {
		return "", fmt.Errorf("file prefix is required for snapshot manifest")
	}
	if item == nil || item.ID == "" {
		return "", fmt.Errorf("job item is required for snapshot manifest")
	}
	if ja.Seaweed == nil || ja.Seaweed.Client == nil {
		return "", fmt.Errorf("seaweed client is not initialized")
	}

	checksum := fileChecksum
	if checksum == "" {
		checksum = item.Checksum
	}
	if checksum == "" {
		var err error
		checksum, err = ja.hashJobItem(ctx, item.ItemPath, item.BackupKey, "")
		if err != nil {
			return "", fmt.Errorf("file checksum for manifest: %w", err)
		}
	}

	rows, err := ja.ChunkRepo.ListItemChunksWithStorage(ctx, item.ID)
	if err != nil {
		return "", err
	}

	inputs := make([]manifest.ChunkInput, len(rows))
	for i, row := range rows {
		inputs[i] = manifest.ChunkInput{Index: row.ChunkIndex, ChunkHash: row.ChunkHash}
	}

	backedUpAt := item.CreatedAt.UTC()
	snap, err := manifest.Build(inputs, manifest.Meta{
		JobItemID:          item.ID,
		ItemPath:           item.ItemPath,
		BackedUpAt:         backedUpAt,
		FileChecksumSHA256: checksum,
		FileSizeBytes:      item.OriginalSize,
		ChunkSizeBytes:     manifest.ChunkSizeBytes,
	})
	if err != nil {
		return "", fmt.Errorf("build snapshot manifest: %w", err)
	}

	body, err := snap.MarshalJSONBytes()
	if err != nil {
		return "", fmt.Errorf("marshal snapshot manifest: %w", err)
	}

	folder := manifest.SnapshotFolderName(item.ID, backedUpAt)
	objectKey := manifest.SnapshotObjectKey(filePrefix, folder)

	_, err = ja.Seaweed.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(ja.Seaweed.Bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("upload snapshot manifest to %s: %w", objectKey, err)
	}

	return objectKey, nil
}

func (ja *JobActivities) PublishSnapshotManifestActivity(ctx context.Context, input types.PublishSnapshotManifestRequest) (types.PublishSnapshotManifestResponse, error) {
	if input.ItemID == "" {
		return types.PublishSnapshotManifestResponse{}, fmt.Errorf("item id is required")
	}
	if input.FileName == "" {
		return types.PublishSnapshotManifestResponse{}, fmt.Errorf("file_name is required")
	}

	item, err := ja.JobItemRepo.GetJobItemByID(ctx, input.ItemID)
	if err != nil {
		return types.PublishSnapshotManifestResponse{}, err
	}
	if item == nil {
		return types.PublishSnapshotManifestResponse{}, fmt.Errorf("job item not found: %s", input.ItemID)
	}

	filePrefix, err := config.SeaweedObjectPrefixFromBackupKey(item.BackupKey, input.FileName)
	if err != nil {
		return types.PublishSnapshotManifestResponse{}, fmt.Errorf("seaweed object prefix: %w", err)
	}
	key, err := ja.publishSnapshotManifest(ctx, item, filePrefix, "")
	if err != nil {
		return types.PublishSnapshotManifestResponse{}, err
	}

	return types.PublishSnapshotManifestResponse{
		ItemID:      input.ItemID,
		ManifestKey: key,
	}, nil
}