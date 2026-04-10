package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectInfo holds metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}

// Destination is the interface for any backup target.
type Destination interface {
	// Put writes an object to the destination.
	Put(ctx context.Context, key string, reader io.Reader, size int64) error
	// Exists checks if the object already exists (for skip_existing).
	Exists(ctx context.Context, key string) (bool, error)
	// String returns a human-readable label.
	String() string
}

// --- Local filesystem destination ---

type LocalDestination struct {
	BasePath string
}

func NewLocalDestination(basePath string) (*LocalDestination, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating base path: %w", err)
	}
	return &LocalDestination{BasePath: basePath}, nil
}

func (d *LocalDestination) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	dest := filepath.Join(d.BasePath, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func (d *LocalDestination) Exists(ctx context.Context, key string) (bool, error) {
	dest := filepath.Join(d.BasePath, key)
	_, err := os.Stat(dest)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (d *LocalDestination) String() string {
	return fmt.Sprintf("local:%s", d.BasePath)
}

// --- S3 destination ---

type S3Destination struct {
	Client *minio.Client
	Bucket string
	Prefix string
}

func NewS3Destination(cfg *S3Config) (*S3Destination, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("creating s3 destination client: %w", err)
	}
	return &S3Destination{Client: client, Bucket: cfg.Bucket, Prefix: cfg.Prefix}, nil
}

func (d *S3Destination) Put(ctx context.Context, key string, reader io.Reader, size int64) error {
	destKey := d.Prefix + key
	_, err := d.Client.PutObject(ctx, d.Bucket, destKey, reader, size, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("uploading to s3: %w", err)
	}
	return nil
}

func (d *S3Destination) Exists(ctx context.Context, key string) (bool, error) {
	destKey := d.Prefix + key
	_, err := d.Client.StatObject(ctx, d.Bucket, destKey, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *S3Destination) String() string {
	return fmt.Sprintf("s3:%s/%s", d.Bucket, d.Prefix)
}
