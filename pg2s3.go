package pg2s3

import (
	"bytes"
	"context"
	"errors"
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
		return "", errors.New("prefix must not contain '_' or '.'")
	}

	timestamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("%s_%s.backup", prefix, timestamp), nil
}

func GenerateBackupPath(name string) string {
	return filepath.Join(os.TempDir(), name)
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

	var capture bytes.Buffer
	cmd.Stdout = &capture
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return errors.New(capture.String())
	}

	return nil
}

// pg_restore -c -d $PG2S3_DATABASE_URL testdata/dvdrental.backup
func (c *Client) RestoreBackup(path string) error {
	args := []string{
		"-c",
		"-d",
		c.pgConnectionURI,
		path,
	}
	cmd := exec.Command("pg_restore", args...)

	var capture bytes.Buffer
	cmd.Stdout = &capture
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return errors.New(capture.String())
	}

	return nil
}

func (c *Client) ListBackups() ([]string, error) {
	client, err := c.connect()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	objects := client.ListObjects(
		ctx,
		c.s3BucketName,
		minio.ListObjectsOptions{})

	var backups []string
	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}
		backups = append(backups, object.Key)
	}

	err = sortBackups(backups)
	if err != nil {
		return nil, err
	}

	return backups, nil
}

func (c *Client) UploadBackup(path, name string) error {
	client, err := c.connect()
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

func (c *Client) DownloadBackup(name, path string) error {
	client, err := c.connect()
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = client.FGetObject(
		ctx,
		c.s3BucketName,
		name,
		path,
		minio.GetObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteBackup(name string) error {
	client, err := c.connect()
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = client.RemoveObject(
		ctx,
		c.s3BucketName,
		name,
		minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) connect() (*minio.Client, error) {
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
		return nil, err
	}

	return client, nil
}

// Parse timestamp by splitting on "_" or "." and parsing the 2nd element
func parseBackupTimestamp(name string) (time.Time, error) {
	delimiters := regexp.MustCompile(`(_|\.)`)

	timestamp := delimiters.Split(name, -1)[1]
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

// sort backups in descending order (newest first, oldest last)
func sortBackups(backups []string) error {
	// pre-check backups for invalid naming
	for _, backup := range backups {
		_, err := parseBackupTimestamp(backup)
		if err != nil {
			return err
		}
	}

	// sort the backups by timestamp
	sort.SliceStable(backups, func (i, j int) bool {
		tI, _ := parseBackupTimestamp(backups[i])
		tJ, _ := parseBackupTimestamp(backups[j])
		return tI.After(tJ)
	})

	return nil
}
