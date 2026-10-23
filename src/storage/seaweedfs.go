package storage

import (
	"context"
	"log"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	log.Printf("Initialized SeaweedFS client targeting endpoint: %s (Bucket: %s)", endpoint, bucket)

	return &SeaweedClient{
		Client: s3Client,
		Bucket: bucket,
	}, nil
}
