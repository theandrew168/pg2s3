package pg2s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"

	"filippo.io/age"
	"github.com/djherbis/buffer"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/theandrew168/pg2s3/internal/config"
)

type Client struct {
	cfg config.Config
}

func NewClient(cfg config.Config) (*Client, error) {
	ctx := context.Background()

	// instantiate a pg2s3 client
	client := &Client{
		cfg: cfg,
	}

	// validate connection to PG
	connPG, err := pgx.Connect(ctx, cfg.PGURL)
	if err != nil {
		return nil, err
	}
	defer connPG.Close(ctx)

	if err = connPG.Ping(ctx); err != nil {
		return nil, err
	}

	// validate connection to S3
	connS3, err := client.connectS3()
	if err != nil {
		return nil, err
	}

	if _, err = connS3.ListBuckets(ctx); err != nil {
		return nil, err
	}

	// validate public keys (if provided)
	for _, pubkey := range cfg.Encryption.PublicKeys {
		_, err = age.ParseX25519Recipient(pubkey)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (c *Client) CreateBackup() (io.Reader, error) {
	args := []string{
		"-Fc", // custom output format (compressed and flexible)
		c.cfg.PGURL,
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
		"-c", // clean DB object before recreating them
		"-d", // database to be restored
		c.cfg.PGURL,
	}
	for _, schema := range c.cfg.Restore.Schemas {
		// specify which schemas should be restored
		args = append(args, "-n", schema)
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

func (c *Client) EncryptBackup(backup io.Reader, publicKeys []string) (io.Reader, error) {
	var recipients []age.Recipient
	for _, pubkey := range publicKeys {
		recipient, err := age.ParseX25519Recipient(pubkey)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, recipient)
	}

	// setup encryption pipeline
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipients...)
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
		c.cfg.S3.BucketName,
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
		c.cfg.S3.BucketName,
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
		c.cfg.S3.BucketName,
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
		c.cfg.S3.BucketName,
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
		c.cfg.S3.AccessKeyID,
		c.cfg.S3.SecretAccessKey,
		"",
	)

	// disable HTTPS requirement for local development / testing
	secure := true
	if strings.Contains(c.cfg.S3.Endpoint, "localhost") {
		secure = false
	}
	if strings.Contains(c.cfg.S3.Endpoint, "127.0.0.1") {
		secure = false
	}

	client, err := minio.New(c.cfg.S3.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}
