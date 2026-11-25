//go:build integration

package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func seaweedIntegrationEnv(t *testing.T) (endpoint, region, accessKey, secretKey string) {
	t.Helper()
	if os.Getenv("SEAWEED_INTEGRATION") != "1" {
		t.Skip("set SEAWEED_INTEGRATION=1 to run Seaweed object-lock integration tests")
	}
	endpoint = strings.TrimSpace(os.Getenv("SEAWEED_ENDPOINT"))
	if endpoint == "" {
		endpoint = "http://localhost:8333"
	}
	accessKey = strings.TrimSpace(os.Getenv("SEAWEED_ACCESS_KEY"))
	secretKey = strings.TrimSpace(os.Getenv("SEAWEED_SECRET_KEY"))
	if accessKey == "" || secretKey == "" {
		t.Skip("SEAWEED_ACCESS_KEY and SEAWEED_SECRET_KEY required")
	}
	region = strings.TrimSpace(os.Getenv("SEAWEED_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	return endpoint, region, accessKey, secretKey
}

// TestWORMComplianceBlocksDeleteBeforeRetainUntil uploads with COMPLIANCE and expects DeleteObjectVersion to be blocked.
func TestWORMComplianceBlocksDeleteBeforeRetainUntil(t *testing.T) {
	endpoint, region, accessKey, secretKey := seaweedIntegrationEnv(t)
	bucket := fmt.Sprintf("worm-it-%d", time.Now().UnixNano())
	ctx := context.Background()

	client, err := InitSeaweedFS(endpoint, region, accessKey, secretKey, bucket, true)
	if err != nil {
		t.Fatalf("init client: %v", err)
	}
	retainUntil := time.Now().UTC().Add(48 * time.Hour)
	key := "integration/locked-object.bin"
	body := []byte("cosmosbdr-worm-integration")

	putRes, err := client.PutObjectLocked(ctx, key, body, "application/octet-stream", retainUntil)
	if err != nil {
		t.Fatalf("PutObjectLocked: %v", err)
	}
	if putRes.VersionID == "" {
		t.Fatal("expected version id from versioned lock-enabled bucket")
	}

	outcome, err := client.DeleteObjectVersion(ctx, key, putRes.VersionID)
	if err != nil {
		t.Fatalf("DeleteObjectVersion: %v", err)
	}
	if outcome.Deleted {
		t.Fatal("expected delete to be blocked by COMPLIANCE retention before retain_until")
	}
	if !outcome.Blocked {
		t.Fatalf("expected Blocked=true, got outcome %+v", outcome)
	}
}

// TestWORMExtendRetention advances retain_until on an existing version (dedup path).
func TestWORMExtendRetention(t *testing.T) {
	endpoint, region, accessKey, secretKey := seaweedIntegrationEnv(t)
	bucket := fmt.Sprintf("worm-it-%d", time.Now().UnixNano())
	ctx := context.Background()

	client, err := InitSeaweedFS(endpoint, region, accessKey, secretKey, bucket, true)
	if err != nil {
		t.Fatalf("init client: %v", err)
	}
	key := "integration/extend-retention.bin"
	firstUntil := time.Now().UTC().Add(24 * time.Hour)
	putRes, err := client.PutObjectLocked(ctx, key, []byte("v1"), "application/octet-stream", firstUntil)
	if err != nil {
		t.Fatalf("PutObjectLocked: %v", err)
	}
	secondUntil := firstUntil.Add(7 * 24 * time.Hour)
	if err := client.ExtendObjectRetention(ctx, key, putRes.VersionID, secondUntil); err != nil {
		t.Fatalf("ExtendObjectRetention: %v", err)
	}
	outcome, err := client.DeleteObjectVersion(ctx, key, putRes.VersionID)
	if err != nil {
		t.Fatalf("DeleteObjectVersion after extend: %v", err)
	}
	if outcome.Deleted || !outcome.Blocked {
		t.Fatalf("expected delete still blocked until extended retain_until, got %+v", outcome)
	}
}