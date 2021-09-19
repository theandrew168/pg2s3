package pg2s3

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

// pg_restore -d $PG2S3_DATABASE_URL testdata/dvdrental.backup
func (pg2s3 *PG2S3) RestoreBackup(path string) error {
	args := []string{
		"-d",
		pg2s3.PGConnectionURI,
		path,
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
