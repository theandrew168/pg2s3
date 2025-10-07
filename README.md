# pg2s3
Simple PostgreSQL backups to S3-compatible storage

## Overview
This project strives to be a simple and reliable backup solution for [PostgreSQL](https://www.postgresql.org/) databases.
In general, pg2s3 dumps a given database and uploads it to an S3-compatible storage bucket.
However, there is a bit more nuance involved in bookkeeping, restoration, and pruning.

### Data Included
The backups created by pg2s3 are data-only and don't include global information such as roles and tablespaces.
To include those would require running pg2s3 as a database superuser which introduces additional security risks.
One of the design goals for pg2s3 was to be as useful as possible without requiring elevated database access.

Instead, it is expected that restores will only ever be ran against databases that are already configured with the necessary roles.
During restoration, any schemes / tables that need to be created will be owned by the user that pg2s3 uses to connect to the database.

This tool is intended for simple database access patterns: where all schems and tables within a database are owned by a single user and have default permissions.
If your use case is more complex than this and you need support for any of the following:
1. Recreating existing roles (users) and modifying table ownership
2. Recreating existing permissions (grants / revokes) on tables

Then pg2s3 might not be suitable for your use case and you should consider something with more features.

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

Note that the S3 bucket defined by `s3_url` must be created outside of this tool.
Bucket creation has more configuration and security options than pg2s3 is positioned to deal with.

Additionally, the value defined by `backup.retention` simply refers to the _number_ of backups kept during a prune.
It has nothing to do with a backup's age or total bucket size.
If `backup.schedule` is set, you'll want to consider the scheduling frequency when determining an appropriate retention count.

The following settings are available for pg2s3:

| Setting            | Required? | Description |
| ------------------ | --------- | ----------- |
| `pg_url`           | Yes       | PostgreSQL connection string |
| `s3_url`           | Yes       | S3-compatible storage connection string |
| `backup.prefix`    | No        | Prefix attached to the name of each backup (default `"pg2s3"`) |
| `backup.retention` | No        | Number of backups to retain after pruning (defaults to keeping all backups) |
| `backup.schedule`  | No        | Backup schedule as a standard cron expression (UTC, required if running in scheduled mode) |
| `restore.schemas`  | No        | List of schemas to restore (defaults to all schemas) |

## Encryption
Backups managed by pg2s3 can be optionally encrypted using [age](https://github.com/FiloSottile/age).
To enable this feature, an age public key must be defined within the config file.
Note that the private key associated with this public key must be kept safe and secure!
When restoring a backup, pg2s3 will prompt for the private key.
This key is intentionally absent from pg2s3's configuration in order to require user intervention for any data decryption.

| Setting                  | Required? | Description |
| ------------------------ | --------- | ----------- |
| `encryption.public_keys` | No        | Public keys for backup encryption |

## Usage
The pg2s3 command-line tool offers three mutually-exclusive actions:
* `pg2s3 backup` - Create a new backup and upload to S3
* `pg2s3 restore` - Download the latest backup from S3 and restore
* `pg2s3 prune` - Prune old backups from S3

If none of these are provided, pg2s3 will attempt to run in scheduled mode: sleeping until `backup.schedule` arrives and then performing a backup + prune.

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
