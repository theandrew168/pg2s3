package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func RequireEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("missing required env var: %s\n", name)
	}
	return value
}

func GenerateBackupName(prefix string) string {
	return prefix + "_" + time.Now().Format(time.RFC3339) + ".backup"
}

func BackupDatabase(path string) error {
	cmdBody := []string{
		"if=/dev/urandom",
		fmt.Sprintf("of=%s", path),
		"bs=1m",
		"count=10",
	}
	cmd := exec.Command("dd", cmdBody...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		log.Print(out.String())
		return err
	}

	return nil
}

func main() {
	endpoint := RequireEnv("PG2S3_ENDPOINT")
	accessKeyID := RequireEnv("PG2S3_ACCESS_KEY_ID")
	secretAccessKey := RequireEnv("PG2S3_SECRET_ACCESS_KEY")
	bucketName := RequireEnv("PG2S3_BUCKET_NAME")
	backupPrefix := RequireEnv("PG2S3_BACKUP_PREFIX")

	log.Println(endpoint)
	log.Println(accessKeyID)
	log.Println(secretAccessKey)
	log.Println(bucketName)

	objectName := GenerateBackupName(backupPrefix)
	filePath := filepath.Join(os.TempDir(), objectName)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatal(err)
	}

	if exists {
		log.Printf("bucket exists: %s\n", bucketName)
	} else {
		err = client.MakeBucket(ctx,
			bucketName,
			minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("successfully created bucket: %s\n", bucketName)
	}

	log.Printf("creating backup: %s\n", objectName)
	err = BackupDatabase(filePath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("uploading backup: %s\n", objectName)
	info, err := client.FPutObject(ctx,
		bucketName,
		objectName,
		filePath,
		minio.PutObjectOptions{})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("uploaded backup: %s (%d bytes)\n", objectName, info.Size)

	// TODO: list all backups
	// TODO: prune old backups
}
