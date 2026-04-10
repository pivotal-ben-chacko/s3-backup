package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Stats tracks backup progress.
type Stats struct {
	Total    int64
	Copied   int64
	Skipped  int64
	Failed   int64
	Bytes    int64
}

// BackupEngine coordinates the backup process.
type BackupEngine struct {
	source  *minio.Client
	srcCfg  S3Config
	dest    Destination
	opts    Options
	logger  *log.Logger
	stats   Stats
}

// NewBackupEngine creates a configured backup engine.
func NewBackupEngine(cfg *Config, dest Destination, logger *log.Logger) (*BackupEngine, error) {
	srcClient, err := minio.New(cfg.Source.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Source.AccessKey, cfg.Source.SecretKey, ""),
		Secure: cfg.Source.UseSSL,
		Region: cfg.Source.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("creating source client: %w", err)
	}

	return &BackupEngine{
		source: srcClient,
		srcCfg: cfg.Source,
		dest:   dest,
		opts:   cfg.Options,
		logger: logger,
	}, nil
}

// Run executes the backup with a worker pool.
func (e *BackupEngine) Run(ctx context.Context) error {
	e.logger.Printf("Starting backup: %s/%s -> %s", e.srcCfg.Bucket, e.srcCfg.Prefix, e.dest)
	e.logger.Printf("Workers: %d | SkipExisting: %v | DryRun: %v", e.opts.Workers, e.opts.SkipExisting, e.opts.DryRun)

	start := time.Now()

	// List objects from source
	objectCh := e.source.ListObjects(ctx, e.srcCfg.Bucket, minio.ListObjectsOptions{
		Prefix:    e.srcCfg.Prefix,
		Recursive: true,
	})

	// Fan out to workers
	jobs := make(chan minio.ObjectInfo, e.opts.Workers*2)
	var wg sync.WaitGroup

	for i := 0; i < e.opts.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			e.worker(ctx, workerID, jobs)
		}(i)
	}

	// Feed objects to workers
	for obj := range objectCh {
		if obj.Err != nil {
			e.logger.Printf("ERROR listing: %v", obj.Err)
			atomic.AddInt64(&e.stats.Failed, 1)
			continue
		}
		atomic.AddInt64(&e.stats.Total, 1)
		jobs <- obj
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	e.logger.Printf("Backup complete in %s", elapsed.Round(time.Millisecond))
	e.logger.Printf("Total: %d | Copied: %d | Skipped: %d | Failed: %d | Bytes: %d",
		e.stats.Total, e.stats.Copied, e.stats.Skipped, e.stats.Failed, e.stats.Bytes)

	if e.stats.Failed > 0 {
		return fmt.Errorf("%d objects failed to backup", e.stats.Failed)
	}
	return nil
}

func (e *BackupEngine) worker(ctx context.Context, id int, jobs <-chan minio.ObjectInfo) {
	for obj := range jobs {
		if err := e.backupObject(ctx, obj); err != nil {
			e.logger.Printf("[worker %d] FAIL %s: %v", id, obj.Key, err)
			atomic.AddInt64(&e.stats.Failed, 1)
		}
	}
}

func (e *BackupEngine) backupObject(ctx context.Context, obj minio.ObjectInfo) error {
	// Strip source prefix to get the relative key
	relKey := obj.Key
	if len(e.srcCfg.Prefix) > 0 {
		relKey = obj.Key[len(e.srcCfg.Prefix):]
	}

	// Skip existing if configured
	if e.opts.SkipExisting {
		exists, err := e.dest.Exists(ctx, relKey)
		if err != nil {
			return fmt.Errorf("checking existence: %w", err)
		}
		if exists {
			atomic.AddInt64(&e.stats.Skipped, 1)
			return nil
		}
	}

	if e.opts.DryRun {
		e.logger.Printf("[dry-run] would copy: %s (%d bytes)", obj.Key, obj.Size)
		atomic.AddInt64(&e.stats.Skipped, 1)
		return nil
	}

	// Download from source with retries
	var lastErr error
	for attempt := 1; attempt <= e.opts.RetryAttempts; attempt++ {
		reader, err := e.source.GetObject(ctx, e.srcCfg.Bucket, obj.Key, minio.GetObjectOptions{})
		if err != nil {
			lastErr = err
			e.logger.Printf("Retry %d/%d for %s: %v", attempt, e.opts.RetryAttempts, obj.Key, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		err = e.dest.Put(ctx, relKey, reader, obj.Size)
		reader.Close()
		if err != nil {
			lastErr = err
			e.logger.Printf("Retry %d/%d for %s: %v", attempt, e.opts.RetryAttempts, obj.Key, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		atomic.AddInt64(&e.stats.Copied, 1)
		atomic.AddInt64(&e.stats.Bytes, obj.Size)
		return nil
	}

	return fmt.Errorf("after %d attempts: %w", e.opts.RetryAttempts, lastErr)
}
