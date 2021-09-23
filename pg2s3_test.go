package pg2s3_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/theandrew168/pg2s3"
)

// TODO: test for missing pg_dump / pg_restore commands?
// TODO: test for failed connection to minio?

const privateKey = "AGE-SECRET-KEY-1L54UFTSF6GUXYQMMQ8HDFYCQ59E7R80RPFLJZS3V3S0M7AFLAD4QUAFH3J"

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

func TestParseBackupTimestamp(t *testing.T) {
	name := "pg2s3_2021-09-23T14:41:17-05:00.backup.age"
	got, err := pg2s3.ParseBackupTimestamp(name)
	if err != nil {
		t.Fatal(err)
	}

	want, err := time.Parse(time.RFC3339, "2021-09-23T14:41:17-05:00")
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatal("mismatched timestamps")
	}

	name = "foobarinvalid.backup"
	_, err = pg2s3.ParseBackupTimestamp(name)
	if err == nil {
		t.Fatal("expected invalid backup name")
	}

	name = "foobar_07131994.backup"
	_, err = pg2s3.ParseBackupTimestamp(name)
	if err == nil {
		t.Fatal("expected invalid backup name")
	}
}

func TestBackup(t *testing.T) {
	pgConnectionURI := requireEnv(t, pg2s3.EnvPGConnectionURI)
	s3Endpoint := requireEnv(t, pg2s3.EnvS3Endpoint)
	s3AccessKeyID := requireEnv(t, pg2s3.EnvS3AccessKeyID)
	s3SecretAccessKey := requireEnv(t, pg2s3.EnvS3SecretAccessKey)
	s3BucketName := requireEnv(t, pg2s3.EnvS3BucketName)

	prefix := requireEnv(t, pg2s3.EnvBackupPrefix)
	_, err := strconv.Atoi(requireEnv(t, pg2s3.EnvBackupRetention))
	if err != nil {
		t.Fatal(err)
	}

	publicKey := os.Getenv(pg2s3.EnvAgePublicKey)

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

	// create backup
	backup, err := client.CreateBackup()
	if err != nil {
		t.Fatal(err)
	}

	// encrypt backup (if applicable)
	if publicKey != "" {
		backup, err = client.EncryptBackup(backup, publicKey)
		if err != nil {
			t.Fatal(err)
		}

		name = name + ".age"
	}

	// upload backup
	err = client.UploadBackup(name, backup)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRestore(t *testing.T) {
	pgConnectionURI := requireEnv(t, pg2s3.EnvPGConnectionURI)
	s3Endpoint := requireEnv(t, pg2s3.EnvS3Endpoint)
	s3AccessKeyID := requireEnv(t, pg2s3.EnvS3AccessKeyID)
	s3SecretAccessKey := requireEnv(t, pg2s3.EnvS3SecretAccessKey)
	s3BucketName := requireEnv(t, pg2s3.EnvS3BucketName)

	_ = requireEnv(t, pg2s3.EnvBackupPrefix)
	_, err := strconv.Atoi(requireEnv(t, pg2s3.EnvBackupRetention))
	if err != nil {
		t.Fatal(err)
	}

	publicKey := os.Getenv(pg2s3.EnvAgePublicKey)

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

	// download backup
	backup, err := client.DownloadBackup(latest)
	if err != nil {
		t.Fatal(err)
	}

	// decrypt backup (if applicable)
	if publicKey != "" {
		backup, err = client.DecryptBackup(backup, privateKey)
		if err != nil {
			t.Fatal(err)
		}
	}

	// restore backup
	err = client.RestoreBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrune(t *testing.T) {
	pgConnectionURI := requireEnv(t, pg2s3.EnvPGConnectionURI)
	s3Endpoint := requireEnv(t, pg2s3.EnvS3Endpoint)
	s3AccessKeyID := requireEnv(t, pg2s3.EnvS3AccessKeyID)
	s3SecretAccessKey := requireEnv(t, pg2s3.EnvS3SecretAccessKey)
	s3BucketName := requireEnv(t, pg2s3.EnvS3BucketName)

	_ = requireEnv(t, pg2s3.EnvBackupPrefix)
	retention, err := strconv.Atoi(requireEnv(t, pg2s3.EnvBackupRetention))
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
