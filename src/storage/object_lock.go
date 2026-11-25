package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// PutObjectResult is Seaweed upload output used for WORM bookkeeping.
type PutObjectResult struct {
	VersionID   string
	RetainUntil time.Time
}

// PutObjectLocked uploads body with COMPLIANCE retention until retainUntil (requires object-lock bucket).
func (c *SeaweedClient) PutObjectLocked(ctx context.Context, objectKey string, body []byte, contentType string, retainUntil time.Time) (PutObjectResult, error) {
	if c == nil || c.Client == nil {
		return PutObjectResult{}, fmt.Errorf("seaweed client is not initialized")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return PutObjectResult{}, fmt.Errorf("object key is required")
	}
	if retainUntil.IsZero() {
		return PutObjectResult{}, fmt.Errorf("retain_until is required for locked upload")
	}
	in := &s3.PutObjectInput{
		Bucket:                    aws.String(c.Bucket),
		Key:                       aws.String(objectKey),
		Body:                      bytes.NewReader(body),
		ObjectLockMode:            types.ObjectLockModeCompliance,
		ObjectLockRetainUntilDate: aws.Time(retainUntil.UTC()),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	out, err := c.Client.PutObject(ctx, in)
	if err != nil {
		return PutObjectResult{}, fmt.Errorf("put object %q: %w", objectKey, err)
	}
	versionID := ""
	if out.VersionId != nil {
		versionID = strings.TrimSpace(*out.VersionId)
	}
	return PutObjectResult{VersionID: versionID, RetainUntil: retainUntil.UTC()}, nil
}

// PutObjectPlain uploads without object lock (legacy / WORM disabled).
func (c *SeaweedClient) PutObjectPlain(ctx context.Context, objectKey string, body io.Reader, contentType string) error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("seaweed client is not initialized")
	}
	in := &s3.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	_, err := c.Client.PutObject(ctx, in)
	if err != nil {
		return fmt.Errorf("put object %q: %w", objectKey, err)
	}
	return nil
}

// ExtendObjectRetention moves COMPLIANCE retain-until forward for one object version.
func (c *SeaweedClient) ExtendObjectRetention(ctx context.Context, objectKey, versionID string, retainUntil time.Time) error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("seaweed client is not initialized")
	}
	objectKey = strings.TrimSpace(objectKey)
	versionID = strings.TrimSpace(versionID)
	if objectKey == "" || versionID == "" {
		return fmt.Errorf("object key and version id are required")
	}
	if retainUntil.IsZero() {
		return fmt.Errorf("retain_until is required")
	}
	_, err := c.Client.PutObjectRetention(ctx, &s3.PutObjectRetentionInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
		Retention: &types.ObjectLockRetention{
			Mode:            types.ObjectLockRetentionModeCompliance,
			RetainUntilDate: aws.Time(retainUntil.UTC()),
		},
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return fmt.Errorf("extend retention %q: %w", objectKey, err)
	}
	return nil
}

// DeleteObjectOutcome reports whether the blob was removed or blocked (WORM).
type DeleteObjectOutcome struct {
	Deleted bool
	Blocked bool // AccessDenied while retention active
	Skipped bool // retain_until in app metadata not yet elapsed
}

// DeleteObjectVersion removes a specific version; missing keys are success.
func (c *SeaweedClient) DeleteObjectVersion(ctx context.Context, objectKey, versionID string) (DeleteObjectOutcome, error) {
	if c == nil || c.Client == nil {
		return DeleteObjectOutcome{}, fmt.Errorf("seaweed client is not initialized")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return DeleteObjectOutcome{}, nil
	}
	in := &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
	}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	_, err := c.Client.DeleteObject(ctx, in)
	if err == nil {
		return DeleteObjectOutcome{Deleted: true}, nil
	}
	if isObjectMissing(err) {
		return DeleteObjectOutcome{Deleted: true}, nil
	}
	if isAccessDenied(err) {
		return DeleteObjectOutcome{Blocked: true}, nil
	}
	return DeleteObjectOutcome{}, fmt.Errorf("delete object %q: %w", objectKey, err)
}

func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AccessDenied" {
		return true
	}
	return false
}