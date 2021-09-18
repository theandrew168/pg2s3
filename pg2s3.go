package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// Backup naming scheme:
// <prefix>_<timestamp>.<ext>[.<ext>]*
func GenerateBackupName(prefix string) string {
	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("%s_%s.backup", prefix, timestamp)
}

func GenerateBackupPath(name string) string {
	return filepath.Join(os.TempDir(), name)
}

// Parse timestamp by splitting on "_" or "." and parsing the 2nd element
func ParseBackupTimestamp(name string) (time.Time, error) {
	delimiters := regexp.MustCompile(`(_|\.)`)

	timestamp := delimiters.Split(name, -1)[1]
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

// sort backups in descending order (newest first, oldest last)
func SortBackups(backups []string) ([]string, error) {
	// pre-check backups for invalid naming
	for _, backup := range backups {
		_, err := ParseBackupTimestamp(backup)
		if err != nil {
			return nil, err
		}
	}

	// make a copy before sorting
	sorted := make([]string, len(backups))
	copy(sorted, backups)

	// sort the backups by timestamp
	sort.SliceStable(sorted, func (i, j int) bool {
		tI, _ := ParseBackupTimestamp(sorted[i])
		tJ, _ := ParseBackupTimestamp(sorted[j])
		return tI.After(tJ)
	})

	return sorted, nil
}

type PG2S3 struct {
	PGConnectionURI   string
	S3Endpoint        string
	S3AccessKeyID     string
	S3SecretAccessKey string
}

// pg_dump -Fc -f dvdrental.backup $PG2S3_DATABASE_URL
func (pg2s3 *PG2S3) CreateBackup(path string) error {
	args := []string{
		"-Fc",
		"-f",
		path,
		pg2s3.PGConnectionURI,
	}
	cmd := exec.Command("pg_dump", args...)

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

// pg_restore testdata/dvdrental.backup -d $PG2S3_DATABASE_URL
func (pg2s3 *PG2S3) RestoreBackup(path string) error {
	args := []string{
		path,
		"-c",
		"-d",
		pg2s3.PGConnectionURI,
	}
	cmd := exec.Command("pg_restore", args...)

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

func (pg2s3 *PG2S3) EnsureBucketExists(bucket string) error {

	creds := credentials.NewStaticV4(
		pg2s3.S3AccessKeyID,
		pg2s3.S3SecretAccessKey,
		"")

	// disable HTTPS requirement for local development / testing
	secure := true
	if strings.Contains(pg2s3.S3Endpoint, "localhost") {
		secure = false
	}
	if strings.Contains(pg2s3.S3Endpoint, "127.0.0.1") {
		secure = false
	}

	client, err := minio.New(pg2s3.S3Endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}

	if !exists {
		err = client.MakeBucket(
			ctx,
			bucket,
			minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (pg2s3 *PG2S3) UploadBackup(bucket, name, path string) error {
	creds := credentials.NewStaticV4(
		pg2s3.S3AccessKeyID,
		pg2s3.S3SecretAccessKey,
		"")

	// disable HTTPS requirement for local development / testing
	secure := true
	if strings.Contains(pg2s3.S3Endpoint, "localhost") {
		secure = false
	}
	if strings.Contains(pg2s3.S3Endpoint, "127.0.0.1") {
		secure = false
	}

	client, err := minio.New(pg2s3.S3Endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = client.FPutObject(ctx,
		bucket,
		name,
		path,
		minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// TODO: check argv[1] for "backup" or "restore"

	pg2s3 := PG2S3{
		PGConnectionURI:   RequireEnv("PG2S3_PG_CONNECTION_URI"),
		S3Endpoint:        RequireEnv("PG2S3_S3_ENDPOINT"),
		S3AccessKeyID:     RequireEnv("PG2S3_S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: RequireEnv("PG2S3_S3_SECRET_ACCESS_KEY"),
	}

	bucket := RequireEnv("PG2S3_BUCKET_NAME")
	prefix := RequireEnv("PG2S3_BACKUP_PREFIX")
	retention := RequireEnv("PG2S3_BACKUP_RETENTION")
	log.Println(retention)

	// ensure bucket exists first to verify connection to S3
	log.Printf("ensuring bucket exists: %s\n", bucket)
	err := pg2s3.EnsureBucketExists(bucket)
	if err != nil {
		log.Fatal(err)
	}

	name := GenerateBackupName(prefix)
	path := GenerateBackupPath(name)

	log.Printf("creating backup: %s\n", name)
	err = pg2s3.CreateBackup(path)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: encrypt the backup (vanilla AES256 or AES256 + HMAC SHA-512?)
	// https://stackoverflow.com/questions/49546567/how-do-you-encrypt-large-files-byte-streams-in-go
	// https://github.com/odeke-em/drive/wiki/End-to-End-Encryption

	log.Printf("uploading backup: %s\n", name)
	err = pg2s3.UploadBackup(bucket, name, path)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: list all backups
	// TODO: prune old backups
}
