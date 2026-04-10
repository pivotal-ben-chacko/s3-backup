# s3backup

A concurrent backup tool for S3-compatible object storage. Backs up buckets to either a local filesystem or another S3-compatible service.

## Features

- **Dual destination support** — local filesystem or another S3-compatible bucket
- **Concurrent transfers** — configurable worker pool for parallel downloads/uploads
- **Skip existing** — incremental backups by skipping objects already at the destination
- **Retry with backoff** — automatic retries on transient failures
- **Dry run mode** — preview what would be copied without transferring anything
- **Graceful shutdown** — Ctrl+C finishes in-progress transfers before exiting
- **Logging** — stdout + optional log file

## Build

```bash
go build -o s3backup .
```

## Usage

```bash
# Using a config file
./s3backup -config config.json

# Dry run to preview
./s3backup -config config.json -dry-run

# Override worker count
./s3backup -config config.json -workers 16
```

## Configuration

Copy one of the example configs and fill in your credentials:

```bash
cp config.local.example.json config.json   # for local backup
cp config.s3.example.json config.json       # for S3-to-S3 backup
cp config.ecs.example.json config.json      # for Dell ECS backup
```

### Source (S3-compatible)

| Field       | Description                                   |
|-------------|-----------------------------------------------|
| `endpoint`  | S3 endpoint (e.g. `s3.amazonaws.com`, `play.min.io`) |
| `bucket`    | Source bucket name                             |
| `prefix`    | Optional key prefix to filter objects          |
| `access_key`| Access key ID                                  |
| `secret_key`| Secret access key                              |
| `region`    | Bucket region                                  |
| `use_ssl`   | Use HTTPS (`true`/`false`)                     |

### Destination

Set `type` to `"local"` or `"s3"`, then fill in the matching block:

**Local:**
```json
"destination": {
  "type": "local",
  "local": { "path": "/backups/my-bucket" }
}
```

**S3:**
```json
"destination": {
  "type": "s3",
  "s3": {
    "endpoint": "s3.us-west-2.amazonaws.com",
    "bucket": "backup-bucket",
    "prefix": "backups/",
    "access_key": "...",
    "secret_key": "...",
    "region": "us-west-2",
    "use_ssl": true
  }
}
```

### Options

| Field            | Default | Description                              |
|------------------|---------|------------------------------------------|
| `workers`        | 4       | Number of concurrent transfer goroutines |
| `dry_run`        | false   | Preview only, don't transfer             |
| `skip_existing`  | false   | Skip objects already at destination      |
| `log_file`       | ""      | Optional file path for logging           |
| `retry_attempts` | 3       | Retries per object on failure            |

## Compatible Services

Works with any S3-compatible provider:

- AWS S3
- MinIO
- Backblaze B2
- Cloudflare R2
- DigitalOcean Spaces
- Wasabi
- Google Cloud Storage (S3 interop)
- Dell ECS
- Ceph/RadosGW
