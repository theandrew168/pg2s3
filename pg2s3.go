package pg2s3

import (
	"context"
	"fmt"
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
func GenerateBackupName(prefix string) (string, error) {
	if strings.ContainsAny(prefix, "_.") {
		return "", fmt.Errorf("prefix must not contain '_' or '.'")
	}

	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("%s_%s.backup", prefix, timestamp), nil
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

type Client struct {
	pgConnectionURI   string
	s3Endpoint        string
	s3AccessKeyID     string
	s3SecretAccessKey string
	s3BucketName      string
}

func New(pgConnectionURI, s3Endpoint, s3AccessKeyID, s3SecretAccessKey, s3BucketName string) (*Client, error) {
	client := Client{
		pgConnectionURI:   pgConnectionURI,
		s3Endpoint:        s3Endpoint,
		s3AccessKeyID:     s3AccessKeyID,
		s3SecretAccessKey: s3SecretAccessKey,
		s3BucketName:      s3BucketName,
	}
	return &client, nil
}

// pg_dump -Fc -f dvdrental.backup $PG2S3_DATABASE_URL
func (c *Client) CreateBackup(path string) error {
	args := []string{
		"-Fc",
		"-f",
		path,
		c.pgConnectionURI,
	}
	cmd := exec.Command("pg_dump", args...)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// pg_restore -d $PG2S3_DATABASE_URL testdata/dvdrental.backup
func (c *Client) RestoreBackup(path string) error {
	args := []string{
		"-d",
		c.pgConnectionURI,
		path,
	}
	cmd := exec.Command("pg_restore", args...)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UploadBackup(path, name string) error {
	creds := credentials.NewStaticV4(
		c.s3AccessKeyID,
		c.s3SecretAccessKey,
		"")

	// disable HTTPS requirement for local development / testing
	secure := true
	if strings.Contains(c.s3Endpoint, "localhost") {
		secure = false
	}
	if strings.Contains(c.s3Endpoint, "127.0.0.1") {
		secure = false
	}

	client, err := minio.New(c.s3Endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = client.FPutObject(
		ctx,
		c.s3BucketName,
		name,
		path,
		minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}
