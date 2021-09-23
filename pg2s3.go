package pg2s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/jackc/pgx/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	EnvPGConnectionURI   = "PG2S3_PG_CONNECTION_URI"
	EnvS3Endpoint        = "PG2S3_S3_ENDPOINT"
	EnvS3AccessKeyID     = "PG2S3_S3_ACCESS_KEY_ID"
	EnvS3SecretAccessKey = "PG2S3_S3_SECRET_ACCESS_KEY"
	EnvS3BucketName      = "PG2S3_S3_BUCKET_NAME"
	EnvBackupPrefix      = "PG2S3_BACKUP_PREFIX"
	EnvBackupRetention   = "PG2S3_BACKUP_RETENTION"
	EnvAgePublicKey      = "PG2S3_AGE_PUBLIC_KEY"
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

// Parse backup timestamp by splitting on "_" or "." and parsing the 2nd element
func ParseBackupTimestamp(name string) (time.Time, error) {
	delimiters := regexp.MustCompile(`(_|\.)`)

	fields := delimiters.Split(name, -1)
	if len(fields) < 3 {
		return time.Time{}, errors.New("invalid backup name")
	}

	timestamp := fields[1]
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, errors.New("invalid backup name")
	}

	return t, nil
}

type Client struct {
	pgConnectionURI   string
	s3Endpoint        string
	s3AccessKeyID     string
	s3SecretAccessKey string
	s3BucketName      string
}

func New(pgConnectionURI, s3Endpoint, s3AccessKeyID, s3SecretAccessKey, s3BucketName string) (*Client, error) {
	// TODO: better use of context here? timeouts?
	ctx := context.Background()

	// validate connection to PG
	connPG, err := pgx.Connect(ctx, pgConnectionURI)
	if err != nil {
		return nil, err
	}
	defer connPG.Close(ctx)

	if err = connPG.Ping(ctx); err != nil {
		return nil, err
	}

	// instantiate a pg2s3 client
	client := &Client{
		pgConnectionURI:   pgConnectionURI,
		s3Endpoint:        s3Endpoint,
		s3AccessKeyID:     s3AccessKeyID,
		s3SecretAccessKey: s3SecretAccessKey,
		s3BucketName:      s3BucketName,
	}

	// validate connection to S3
	connS3, err := client.connectS3()
	if err != nil {
		return nil, err
	}

	if _, err = connS3.ListBuckets(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) CreateBackup() (io.Reader, error) {
	args := []string{
		"-Fc",
		c.pgConnectionURI,
	}
	cmd := exec.Command("pg_dump", args...)

	var backup bytes.Buffer
	cmd.Stdout = &backup

	var capture bytes.Buffer
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return nil, errors.New(capture.String())
	}

	return &backup, nil
}

func (c *Client) RestoreBackup(backup io.Reader) error {
	args := []string{
		"-c",
		"-d",
		c.pgConnectionURI,
	}
	cmd := exec.Command("pg_restore", args...)

	cmd.Stdin = backup

	var capture bytes.Buffer
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return errors.New(capture.String())
	}

	return nil
}

func (c *Client) EncryptBackup(backup io.Reader, publicKey string) (io.Reader, error) {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return nil, err
	}

	// setup encryption pipeline
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return nil, err
	}

	// apply encryption by copying data through
	if _, err = io.Copy(w, backup); err != nil {
		return nil, err
	}

	// explicit close to flush encryption
	if err = w.Close(); err != nil {
		return nil, err
	}

	return &encrypted, nil
}

func (c *Client) DecryptBackup(encrypted io.Reader, privateKey string) (io.Reader, error) {
	identity, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return nil, err
	}

	// setup decryption pipeline
	r, err := age.Decrypt(encrypted, identity)
	if err != nil {
		return nil, err
	}

	// apply decryption by copying data through
	var backup bytes.Buffer
	if _, err = io.Copy(&backup, r); err != nil {
		return nil, err
	}

	return &backup, nil
}

func (c *Client) ListBackups() ([]string, error) {
	client, err := c.connectS3()
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

func (c *Client) UploadBackup(name string, backup io.Reader) error {
	client, err := c.connectS3()
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = client.PutObject(
		ctx,
		c.s3BucketName,
		name,
		backup,
		-1,
		minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) DownloadBackup(name string) (io.Reader, error) {
	client, err := c.connectS3()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	backup, err := client.GetObject(
		ctx,
		c.s3BucketName,
		name,
		minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return backup, nil
}

func (c *Client) DeleteBackup(name string) error {
	client, err := c.connectS3()
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

func (c *Client) connectS3() (*minio.Client, error) {
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

// sort backups in descending order (newest first, oldest last)
func sortBackups(backups []string) error {
	// pre-check backups for invalid naming
	for _, backup := range backups {
		_, err := ParseBackupTimestamp(backup)
		if err != nil {
			return err
		}
	}

	// sort the backups by timestamp
	sort.SliceStable(backups, func(i, j int) bool {
		tI, _ := ParseBackupTimestamp(backups[i])
		tJ, _ := ParseBackupTimestamp(backups[j])
		return tI.After(tJ)
	})

	return nil
}
