package vectorfs

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
)

// S3Config holds S3 configuration
type S3Config struct {
	AccessKey string
	SecretKey string
	Bucket    string
	KeyPrefix string
	Region    string
	Endpoint  string
}

// S3Client handles S3 operations for document storage
type S3Client struct {
	client    *s3.Client
	bucket    string
	keyPrefix string
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg S3Config) (*S3Client, error) {
	ctx := context.Background()

	var awsCfg aws.Config
	var err error

	// Configure AWS SDK
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		// Use static credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (IAM role, environment, etc.)
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	var s3Client *s3.Client
	if cfg.Endpoint != "" {
		// Custom endpoint (e.g., MinIO, LocalStack)
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	} else {
		s3Client = s3.NewFromConfig(awsCfg)
	}

	log.Infof("[vectorfs/s3] Initialized S3 client for bucket: %s, prefix: %s", cfg.Bucket, cfg.KeyPrefix)

	return &S3Client{
		client:    s3Client,
		bucket:    cfg.Bucket,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

// buildKey constructs the S3 key: keyPrefix/namespace/digest
func (c *S3Client) buildKey(namespace, digest string) string {
	return fmt.Sprintf("%s/%s/%s", c.keyPrefix, namespace, digest)
}

// UploadDocument uploads a document to S3
func (c *S3Client) UploadDocument(ctx context.Context, namespace, digest string, data []byte) error {
	key := c.buildKey(namespace, digest)

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	log.Debugf("[vectorfs/s3] Uploaded document: %s", key)
	return nil
}

// DownloadDocument downloads a document from S3
func (c *S3Client) DownloadDocument(ctx context.Context, namespace, digest string) ([]byte, error) {
	key := c.buildKey(namespace, digest)

	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object body: %w", err)
	}

	log.Debugf("[vectorfs/s3] Downloaded document: %s (%d bytes)", key, len(data))
	return data, nil
}

// DocumentExists checks if a document exists in S3
func (c *S3Client) DocumentExists(ctx context.Context, namespace, digest string) (bool, error) {
	key := c.buildKey(namespace, digest)

	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if error is "not found"
		return false, nil
	}

	return true, nil
}

// DeleteDocument deletes a document from S3
func (c *S3Client) DeleteDocument(ctx context.Context, namespace, digest string) error {
	key := c.buildKey(namespace, digest)

	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	log.Debugf("[vectorfs/s3] Deleted document: %s", key)
	return nil
}
