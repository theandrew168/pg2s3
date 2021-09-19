package main

import (
	"log"
	"os"
	"strconv"

	"github.com/theandrew168/pg2s3"
)

func main() {
	client, err := pg2s3.New(
		requireEnv("PG2S3_PG_CONNECTION_URI"),
		requireEnv("PG2S3_S3_ENDPOINT"),
		requireEnv("PG2S3_S3_ACCESS_KEY_ID"),
		requireEnv("PG2S3_S3_SECRET_ACCESS_KEY"),
		requireEnv("PG2S3_S3_BUCKET_NAME"))
	if err != nil {
		log.Fatalln(err)
	}

	prefix := requireEnv("PG2S3_BACKUP_PREFIX")
	retention, err := strconv.Atoi(requireEnv("PG2S3_BACKUP_RETENTION"))
	if err != nil {
		log.Fatalln(err)
	}

	usage := "usage: pg2s3 backup|restore|prune"
	if len(os.Args) < 2 {
		log.Fatalln(usage)
	}

	cmd := os.Args[1]
	switch cmd {
	case "backup":
		err = backup(client, prefix, retention)
		if err != nil {
			log.Fatalln(err)
		}
	case "restore":
		err = restore(client, prefix, retention)
		if err != nil {
			log.Fatalln(err)
		}
	case "prune":
		err = prune(client, prefix, retention)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalln(usage)
	}
}

func requireEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("missing required env var: %s\n", name)
	}
	return value
}

func backup(client *pg2s3.Client, prefix string, retention int) error {
	// generate name for backup
	name, err := pg2s3.GenerateBackupName(prefix)
	if err != nil {
		return err
	}

	// generate path for backup
	path := pg2s3.GenerateBackupPath(name)

	// create backup
	err = client.CreateBackup(path)
	if err != nil {
		return err
	}

	// upload backup
	err = client.UploadBackup(path, name)
	if err != nil {
		return err
	}

	// delete backup (from local filesystem)
	err = os.Remove(path)
	if err != nil {
		return err
	}

	return nil
}

func restore(client *pg2s3.Client, prefix string, retention int) error {
	// TODO: list all backups
	backups, err := client.ListBackups()
	if err != nil {
		return err
	}
	for _, backup := range backups {
		log.Println(backup)
	}

	// TODO: determine most recent backup
	// TODO: generate path for backup
	// TODO: download backup
	// TODO: restore backup
	// TODO: delete backup (from local filesystem)
	return nil
}

func prune(client *pg2s3.Client, prefix string, retention int) error {
	// TODO: list all backups
	// TODO: determine old backups to prune
	// TODO: prune old backups
	return nil
}
