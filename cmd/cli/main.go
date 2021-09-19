package main

import (
	"log"
	"os"

	"github.com/theandrew168/pg2s3"
)

func RequireEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("missing required env var: %s\n", name)
	}
	return value
}

func main() {
	// TODO: check argv[1] for "backup", "restore", or "prune"

	tool := pg2s3.PG2S3{
		PGConnectionURI:   RequireEnv("PG2S3_PG_CONNECTION_URI"),
		S3Endpoint:        RequireEnv("PG2S3_S3_ENDPOINT"),
		S3AccessKeyID:     RequireEnv("PG2S3_S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: RequireEnv("PG2S3_S3_SECRET_ACCESS_KEY"),
	}

	bucket := RequireEnv("PG2S3_BUCKET_NAME")
	prefix := RequireEnv("PG2S3_BACKUP_PREFIX")
	retention := RequireEnv("PG2S3_BACKUP_RETENTION")
	log.Println(retention)

	name := pg2s3.GenerateBackupName(prefix)
	path := pg2s3.GenerateBackupPath(name)

	log.Printf("creating backup: %s\n", name)
	err := tool.CreateBackup(path)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: encrypt the backup (vanilla AES256 or AES256 + HMAC SHA-512?)
	// https://stackoverflow.com/questions/49546567/how-do-you-encrypt-large-files-byte-streams-in-go
	// https://github.com/odeke-em/drive/wiki/End-to-End-Encryption

	log.Printf("uploading backup: %s\n", name)
	err = tool.UploadBackup(bucket, name, path)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: list all backups
	// TODO: prune old backups
}
