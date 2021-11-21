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
	"github.com/BurntSushi/toml"
	"github.com/djherbis/buffer"
	"github.com/jackc/pgx/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	PGConnectionURI   string `toml:"pg_connection_uri"`
	S3Endpoint        string `toml:"s3_endpoint"`
	S3AccessKeyID     string `toml:"s3_access_key_id"`
	S3SecretAccessKey string `toml:"s3_secret_access_key"`
	S3BucketName      string `toml:"s3_bucket_name"`
	AgePublicKey      string `toml:"age_public_key"`
	BackupPrefix      string `toml:"backup_prefix"`
	BackupRetention   int    `toml:"backup_retention"`
}

func ReadConfig(path string) (Config, error) {
	var cfg Config
	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, err
	}

	// build set of present config keys
	present := make(map[string]bool)
	for _, keys := range meta.Keys() {
		key := keys[0]
		present[key] = true
	}

	required := []string{
		"pg_connection_uri",
		"s3_endpoint",
		"s3_access_key_id",
		"s3_secret_access_key",
		"s3_bucket_name",
		"backup_prefix",
	}

	// ensure required keys are present
	missing := []string{}
	for _, key := range required {
		if _, ok := present[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		msg := strings.Join(missing, ", ")
		return Config{}, fmt.Errorf("missing config values: %s", msg)
	}

	return cfg, nil
}

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
	cfg Config
}

func New(cfg Config) (*Client, error) {
	// TODO: better use of context here? timeouts?
	ctx := context.Background()

	// validate connection to PG
	connPG, err := pgx.Connect(ctx, cfg.PGConnectionURI)
	if err != nil {
		return nil, err
	}
	defer connPG.Close(ctx)

	if err = connPG.Ping(ctx); err != nil {
		return nil, err
	}

	// instantiate a pg2s3 client
	client := &Client{
		cfg: cfg,
	}

	// validate connection to S3
	connS3, err := client.connectS3()
	if err != nil {
		return nil, err
	}

	if _, err = connS3.ListBuckets(ctx); err != nil {
		return nil, err
	}

	// validate public key (if provided)
	if cfg.AgePublicKey != "" {
		_, err = age.ParseX25519Recipient(cfg.AgePublicKey)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (c *Client) CreateBackup() (io.Reader, error) {
	args := []string{
		"-Fc",
		c.cfg.PGConnectionURI,
	}
	cmd := exec.Command("pg_dump", args...)

	// buffer 32MB to memory, after that buffer to 64MB chunked files
	backup := buffer.NewUnboundedBuffer(32*1024*1024, 64*1024*1024)
	cmd.Stdout = backup

	var capture bytes.Buffer
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return nil, errors.New(capture.String())
	}

	return backup, nil
}

func (c *Client) RestoreBackup(backup io.Reader) error {
	args := []string{
		"-c",
		"-d",
		c.cfg.PGConnectionURI,
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
		c.cfg.S3BucketName,
		minio.ListObjectsOptions{},
	)

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
		c.cfg.S3BucketName,
		name,
		backup,
		-1,
		minio.PutObjectOptions{},
	)
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
		c.cfg.S3BucketName,
		name,
		minio.GetObjectOptions{},
	)
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
		c.cfg.S3BucketName,
		name,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) connectS3() (*minio.Client, error) {
	creds := credentials.NewStaticV4(
		c.cfg.S3AccessKeyID,
		c.cfg.S3SecretAccessKey,
		"",
	)

	// disable HTTPS requirement for local development / testing
	secure := true
	if strings.Contains(c.cfg.S3Endpoint, "localhost") {
		secure = false
	}
	if strings.Contains(c.cfg.S3Endpoint, "127.0.0.1") {
		secure = false
	}

	client, err := minio.New(c.cfg.S3Endpoint, &minio.Options{
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
