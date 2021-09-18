# pg2s3
Simple PostgreSQL backups to S3-compatible storage

## Install / Setup
These vars are required to run pg2s3:
```
PG2S3_DB_CONNECTION_URI
PG2S3_S3_ENDPOINT
PG2S3_S3_ACCESS_KEY_ID
PG2S3_S3_SECRET_ACCESS_KEY
PG2S3_BUCKET_NAME
PG2S3_BACKUP_PREFIX
```

## Testing
To develop and test locally, containers for [PostgreSQL](https://www.postgresql.org/) and [MinIO](https://min.io/) must be running:
```
docker compose up -d
```

These containers can be stopped via:
```
docker compose down
```
