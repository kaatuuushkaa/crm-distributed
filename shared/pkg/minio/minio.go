package minio

import (
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log/slog"
	"net/url"
	"time"
)

type Config struct {
	Endpoint  string `env:"MINIO_ENDPOINT"   envDefault:"localhost:9000"`
	AccessKey string `env:"MINIO_ACCESS_KEY" envDefault:"minioadmin"`
	SecretKey string `env:"MINIO_SECRET_KEY" envDefault:"minioadmin"`
	Bucket    string `env:"MINIO_BUCKET"     envDefault:"crm-documents"`
	UseSSL    bool   `env:"MINIO_USE_SSL"    envDefault:"false"`
	Region    string `env:"MINIO_REGION"     envDefault:"us-east-1"`
}

type Client struct {
	mc     *minio.Client
	bucket string
	log    *slog.Logger
}

func New(ctx context.Context, cfg Config, log *slog.Logger) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	c := &Client{mc: mc, bucket: cfg.Bucket, log: log}

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}

	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
		log.Info("minio bucket created", "bucket", cfg.Bucket)
	}

	log.Info("minio connected", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
	return c, nil
}

func (c *Client) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}

	c.log.DebugContext(ctx, "object uploaded", "key", key, "size", size)
	return nil
}

func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("get object %s: %w", key, err)
	}

	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()

		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return nil, 0, errors.New("object not found")
		}

		return nil, 0, fmt.Errorf("stat object %s: %w", key, err)
	}

	return obj, stat.Size, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}

	c.log.DebugContext(ctx, "object deleted", "key", key)
	return nil
}

func (c *Client) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)

	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expires, reqParams)
	if err != nil {
		return "", fmt.Errorf("presigned url for %s: %w", key, err)
	}

	return u.String(), nil
}

func (c *Client) Check(ctx context.Context) error {
	_, err := c.mc.BucketExists(ctx, c.bucket)
	return err
}
