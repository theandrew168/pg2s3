# pg2s3
Simple PostgreSQL backups to S3-compatible storage

## Overview
This project strives to be a simple and reliable backup solution for [PostgreSQL](https://www.postgresql.org/) databases.
In general, pg2s3 dumps a given database and uploads it to an S3-compatible storage bucket.
However, there is a bit more nuance involved in bookkeeping, restoration, and pruning.

## Install
The pg2s3 tool is distributed as a single, static binary for all major platforms.
It is also released as a `.deb` for Debian-based Linux environments.
Check the [releases page](https://github.com/theandrew168/pg2s3/releases) to find and download the latest version.

Additionally, the environment where pg2s3 is executed must have `pg_dump` and `pg_restore` installed.
These tools are part of the collection of [PostgreSQL Client Applications](https://www.postgresql.org/docs/12/reference-client.html).
On an Ubuntu server, these tools are contained within the package `postgresql-client-<version>` based on the major version of PostgreSQL being used.

## Configuration
Configuration for pg2s3 is handled exclusively through a config file written in [TOML](https://github.com/toml-lang/toml).
By default, pg2s3 will look for a config file named `pg2s3.conf` in the current directory.
This file can be overridden by using the `-conf` flag.

Note that the S3 bucket defined by `s3_bucket_name` must be created outside of this tool.
Bucket creation has more configuration and security options than pg2s3 is positioned to deal with.

Additionally, the value defined by `backup_retention` simply refers to the _number_ of backups kept during a prune.
It has nothing to do with a backup's age or total bucket size.
If `backups_schedule` is set, you'll want to consider the scheduling frequency when determining an appropriate retention count.

The following settings are required to run pg2s3:

| Setting                | Required? | Description |
| ---------------------- | --------- | ----------- |
| `pg_connection_uri`    | Yes       | PostgreSQL connection string |
| `s3_endpoint`          | Yes       | S3-compatible storage endpoint |
| `s3_access_key_id`     | Yes       | S3-compatible storage access key ID |
| `s3_secret_access_key` | Yes       | S3-compatible storage secret access key |
| `s3_bucket_name`       | Yes       | S3-compatible storage bucket name |
| `backup_prefix`        | No        | Prefix attached to the name of each backup (default "pg2s3") |
| `backup_retention`     | No        | Number of backups to retain after pruning |
| `backup_schedule`      | No        | Backup schedule as a standard cron expression (UTC) |
| `restore_schemas`      | No        | List of schemas to restore (default "public") |

## Encryption
Backups managed by pg2s3 can be optionally encrypted using [age](https://github.com/FiloSottile/age).
To enable this feature, an age public key must be defined within the config file.
Note that the private key associated with this public key must be kept safe and secure!
When restoring a backup, pg2s3 will prompt for the private key.
This key is intentionally absent from pg2s3's configuration in order to require user intervention for any data decryption.

| Setting          | Required? | Description |
| ---------------- | --------- | ----------- |
| `age_public_key` | No        | Public key for backup encryption |

## Usage
The pg2s3 command-line tool offers three mutually-exclusive actions:
* `pg2s3 -backup` - Create a new backup and upload to S3
* `pg2s3 -restore` - Download the latest backup from S3 and restore
* `pg2s3 -prune` - Prune old backups from S3

If none of these are provided, pg2s3 will attempt to run in scheduled mode: sleeping until `backup_schedule` arrives and then performing a backup + prune.

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
go test ./...
```
