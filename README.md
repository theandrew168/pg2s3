# pg2s3
Simple PostgreSQL backups to S3-compatible storage

## Install / Setup
The following environment variables are required to run pg2s3:

| Variable                     | Description |
| ---------------------------- | ----------- |
| `PG2S3_PG_CONNECTION_URI`    | PostgreSQL connection string |
| `PG2S3_S3_ENDPOINT`          | S3-compatible storage endpoint |
| `PG2S3_S3_ACCESS_KEY_ID`     | S3-compatible storage access key ID |
| `PG2S3_S3_SECRET_ACCESS_KEY` | S3-compatible storage secret access key |
| `PG2S3_S3_BUCKET_NAME`       | S3-compatible storage bucket name |
| `PG2S3_BACKUP_PREFIX`        | Prefix prepended to the name of each backup |
| `PG2S3_BACKUP_RETENTION`     | Number of backups to retain (after pruning) |

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
