package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	minio          *minio.Client
	bucket         string
	endpoint       string
	publicEndpoint string
	useSSL         bool
}

type Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

func NewClient(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			cfg.AccessKey, cfg.SecretKey, "",
		),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	return &Client{
		minio:          mc,
		bucket:         cfg.Bucket,
		endpoint:       cfg.Endpoint,
		publicEndpoint: cfg.PublicEndpoint,
		useSSL:         cfg.UseSSL,
	}, nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	if c.minio == nil {
		return fmt.Errorf("minio client is nil")
	}

	exists, err := c.minio.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}

	if err := c.minio.MakeBucket(
		ctx, c.bucket, minio.MakeBucketOptions{},
	); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}

	policy := fmt.Sprintf(
		`{"Version":"2012-10-17","Statement":[{"Effect":"Allow",`+
			`"Principal":{"AWS":["*"]},"Action":["s3:GetObject"],`+
			`"Resource":["arn:aws:s3:::%s/*"]}]}`,
		c.bucket,
	)
	if err := c.minio.SetBucketPolicy(
		ctx, c.bucket, policy,
	); err != nil {
		slog.Warn("set bucket policy failed", "error", err)
	}

	return nil
}

func (c *Client) Upload(
	ctx context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) (string, error) {
	_, err := c.minio.PutObject(
		ctx, c.bucket, key, reader, size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	return c.buildURL(key), nil
}

func (c *Client) buildURL(key string) string {
	if strings.Contains(c.publicEndpoint, "://") {
		return fmt.Sprintf(
			"%s/%s/%s",
			strings.TrimRight(c.publicEndpoint, "/"),
			c.bucket,
			key,
		)
	}

	endpoint := c.endpoint
	if c.publicEndpoint != "" {
		endpoint = c.publicEndpoint
	}

	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf(
		"%s://%s/%s/%s", scheme, endpoint, c.bucket, key,
	)
}
