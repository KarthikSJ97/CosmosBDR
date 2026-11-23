package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// SeaweedClient wraps the S3 client configured for SeaweedFS
type SeaweedClient struct {
	Client *s3.Client
	Bucket string
}

// InitSeaweedFS initializes an S3 client pointing to the SeaweedFS S3 endpoint using modern service-specific endpoints
func InitSeaweedFS(endpoint, region, accessKey, secretKey, bucket string) (*SeaweedClient, error) {
	ctx := context.TODO()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	// Initialize S3 client with BaseEndpoint and path-style addressing for SeaweedFS
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})

	client := &SeaweedClient{
		Client: s3Client,
		Bucket: bucket,
	}

	if err := client.EnsureBucket(ctx); err != nil {
		return nil, err
	}

	log.Printf("Initialized SeaweedFS client targeting endpoint: %s (Bucket: %s)", endpoint, bucket)

	return client, nil
}

// EnsureBucket verifies the configured bucket exists and creates it when missing (e.g. after docker compose down -v).
func (c *SeaweedClient) EnsureBucket(ctx context.Context) error {
	if c.Bucket == "" {
		return fmt.Errorf("seaweed bucket name is required (set SEAWEED_BUCKET)")
	}

	_, err := c.Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.Bucket),
	})
	if err == nil {
		log.Printf("SeaweedFS bucket %q is available", c.Bucket)
		return nil
	}
	if !isBucketMissing(err) {
		return fmt.Errorf("check SeaweedFS bucket %q: %w", c.Bucket, err)
	}

	log.Printf("SeaweedFS bucket %q not found, creating...", c.Bucket)
	_, err = c.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(c.Bucket),
	})
	if err == nil {
		log.Printf("SeaweedFS bucket %q created", c.Bucket)
		return nil
	}
	if isBucketAlreadyExists(err) {
		log.Printf("SeaweedFS bucket %q already exists", c.Bucket)
		return nil
	}
	return fmt.Errorf("create SeaweedFS bucket %q: %w", c.Bucket, err)
}

func isBucketMissing(err error) bool {
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket":
			return true
		}
	}
	return false
}

// DeleteObject removes one object from the configured bucket. Missing keys are treated as success.
func (c *SeaweedClient) DeleteObject(ctx context.Context, objectKey string) error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("seaweed client is not initialized")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil
	}
	_, err := c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil && !isObjectMissing(err) {
		return fmt.Errorf("delete SeaweedFS object %q: %w", objectKey, err)
	}
	return nil
}

func isObjectMissing(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	return false
}

func isBucketAlreadyExists(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "BucketAlreadyExists", "BucketAlreadyOwnedByYou":
			return true
		}
	}
	return false
}
