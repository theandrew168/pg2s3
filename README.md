# pg2s3
Simple PostgreSQL backups to S3-compatible storage

## Overview
This project strives to be simple and reliable backup solution for [PostgreSQL](https://www.postgresql.org/) databases.
In general, this code dumps a given database and uploads it to an S3-compatible storage bucket.
However, there is a bit more nuance involved in bookkeeping, restoration, and pruning.

That being said, some features are intentionally left out of scope for this project.
For example, PostgreSQL is the only supported database and S3 is the only supported storage method.
The scheduling of periodic backups is also left out: rely on tools such as [cron](https://wiki.archlinux.org/title/cron) or [systemd timers](https://wiki.archlinux.org/title/Systemd/Timers) to handle the timing and frequency of programmatic backups.


talk about retention count (not based on date or size, just count)  

## Configuration
Configuration for pg2s3 is handle exclusively through environment variables.
This leaves out the need for config files, command line parameters, and the precedence rules that exist between them.

Note that the S3 bucket noted by `PG2S3_S3_BUCKET_NAME` must be created outside of this tool.
Bucket creation has more configuration and security options than pg2s3 is positioned to deal with.

The following environment variables are required to run pg2s3:

| Variable                     | Description |
| ---------------------------- | ----------- |
| `PG2S3_PG_CONNECTION_URI`    | PostgreSQL connection string |
| `PG2S3_S3_ENDPOINT`          | S3-compatible storage endpoint |
| `PG2S3_S3_ACCESS_KEY_ID`     | S3-compatible storage access key ID |
| `PG2S3_S3_SECRET_ACCESS_KEY` | S3-compatible storage secret access key |
| `PG2S3_S3_BUCKET_NAME`       | S3-compatible storage bucket name |
| `PG2S3_BACKUP_PREFIX`        | Prefix attached to the name of each backup |
| `PG2S3_BACKUP_RETENTION`     | Number of backups to retain after pruning |

## Usage
The pg2s3 command-line tool offers three commands:
* `pg2s3 backup` - Create a new backup from PG and upload to S3
* `pg2s3 restore` - Download the latest backup from S3 and restore to PG
* `pg2s3 prune` - Prune old backups from S3

## Local Development
To develop and test locally, containers for [PostgreSQL](https://www.postgresql.org/) and [MinIO](https://min.io/) must be running:
```
docker compose up -d
```

These containers can be stopped via:
```
docker compose down
```

## Testing
With the above containers running:
```
source testdata/environment
go test ./...
```
