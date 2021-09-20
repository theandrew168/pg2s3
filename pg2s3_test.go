package pg2s3_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/theandrew168/pg2s3"
)

func requireEnv(t *testing.T, name string) string {
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("missing required env var: %s\n", name)
	}
	return value
}

func createBucket(s3Endpoint, s3AccessKeyID, s3SecretAccessKey, s3BucketName string) error {
	creds := credentials.NewStaticV4(
		s3AccessKeyID,
		s3SecretAccessKey,
		"")

	client, err := minio.New(s3Endpoint, &minio.Options{
		Creds:  creds,
		Secure: false,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, s3BucketName)
	if err != nil {
		return err
	}

	if !exists {
		err = client.MakeBucket(
			ctx,
			s3BucketName,
			minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: test for missing pg_dump / pg_restore commands?

func TestGenerateBackupName(t *testing.T) {
	prefix := "pg2s3"
	suffix := ".backup"
	name, err := pg2s3.GenerateBackupName(prefix)
	if err != nil {
		t.Fatal(err)
	}

	// success cases
	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q is missing prefix %q", name, prefix)
	}
	if !strings.HasSuffix(name, suffix) {
		t.Errorf("name %q is missing suffix %q", name, suffix)
	}

	// error cases
	prefix = "foo_bar"
	_, err = pg2s3.GenerateBackupName(prefix)
	if err == nil {
		t.Errorf("prefix %q should be invalid", prefix)
	}

	prefix = "foo.bar"
	_, err = pg2s3.GenerateBackupName(prefix)
	if err == nil {
		t.Errorf("prefix %q should be invalid", prefix)
	}
}

func TestBackup(t *testing.T) {
	pgConnectionURI := requireEnv(t, "PG2S3_PG_CONNECTION_URI")
	s3Endpoint := requireEnv(t, "PG2S3_S3_ENDPOINT")
	s3AccessKeyID := requireEnv(t, "PG2S3_S3_ACCESS_KEY_ID")
	s3SecretAccessKey := requireEnv(t, "PG2S3_S3_SECRET_ACCESS_KEY")
	s3BucketName := requireEnv(t, "PG2S3_S3_BUCKET_NAME")

	prefix := requireEnv(t, "PG2S3_BACKUP_PREFIX")
	_, err := strconv.Atoi(requireEnv(t, "PG2S3_BACKUP_RETENTION"))
	if err != nil {
		t.Fatal(err)
	}

	err = createBucket(s3Endpoint, s3AccessKeyID, s3SecretAccessKey, s3BucketName)
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.New(
		pgConnectionURI,
		s3Endpoint,
		s3AccessKeyID,
		s3SecretAccessKey,
		s3BucketName)
	if err != nil {
		t.Fatal(err)
	}

	// generate name for backup
	name, err := pg2s3.GenerateBackupName(prefix)
	if err != nil {
		t.Fatal(err)
	}

	// generate path for backup
	path := pg2s3.GenerateBackupPath(name)

	// create backup
	err = client.CreateBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	// upload backup
	err = client.UploadBackup(path, name)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRestore(t *testing.T) {
	pgConnectionURI := requireEnv(t, "PG2S3_PG_CONNECTION_URI")
	s3Endpoint := requireEnv(t, "PG2S3_S3_ENDPOINT")
	s3AccessKeyID := requireEnv(t, "PG2S3_S3_ACCESS_KEY_ID")
	s3SecretAccessKey := requireEnv(t, "PG2S3_S3_SECRET_ACCESS_KEY")
	s3BucketName := requireEnv(t, "PG2S3_S3_BUCKET_NAME")

	_ = requireEnv(t, "PG2S3_BACKUP_PREFIX")
	_, err := strconv.Atoi(requireEnv(t, "PG2S3_BACKUP_RETENTION"))
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.New(
		pgConnectionURI,
		s3Endpoint,
		s3AccessKeyID,
		s3SecretAccessKey,
		s3BucketName)
	if err != nil {
		t.Fatal(err)
	}

	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		t.Fatal(err)
	}

	if len(backups) == 0 {
		t.Fatal("no backups present to restore")
	}

	// determine latest backup
	latest := backups[0]

	// generate path for backup
	path := pg2s3.GenerateBackupPath(latest)

	// download backup
	err = client.DownloadBackup(latest, path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	// restore backup
	err = client.RestoreBackup(path)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrune(t *testing.T) {
	pgConnectionURI := requireEnv(t, "PG2S3_PG_CONNECTION_URI")
	s3Endpoint := requireEnv(t, "PG2S3_S3_ENDPOINT")
	s3AccessKeyID := requireEnv(t, "PG2S3_S3_ACCESS_KEY_ID")
	s3SecretAccessKey := requireEnv(t, "PG2S3_S3_SECRET_ACCESS_KEY")
	s3BucketName := requireEnv(t, "PG2S3_S3_BUCKET_NAME")

	_ = requireEnv(t, "PG2S3_BACKUP_PREFIX")
	retention, err := strconv.Atoi(requireEnv(t, "PG2S3_BACKUP_RETENTION"))
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.New(
		pgConnectionURI,
		s3Endpoint,
		s3AccessKeyID,
		s3SecretAccessKey,
		s3BucketName)
	if err != nil {
		t.Fatal(err)
	}

	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		t.Fatal(err)
	}

	// check if backup count exceeds retention
	if len(backups) < retention {
		return
	}

	// determine expired backups to prune
	expired := backups[retention:]

	// prune old backups
	for _, backup := range expired {
		err = client.DeleteBackup(backup)
		if err != nil {
			t.Fatal(err)
		}
	}
}
