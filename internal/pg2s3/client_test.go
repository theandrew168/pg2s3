package pg2s3_test

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/theandrew168/pg2s3/internal/config"
	"github.com/theandrew168/pg2s3/internal/pg2s3"
)

// TODO: test for missing pg_dump / pg_restore commands?
// TODO: test for failed connection to minio?

const privateKey = "AGE-SECRET-KEY-1L54UFTSF6GUXYQMMQ8HDFYCQ59E7R80RPFLJZS3V3S0M7AFLAD4QUAFH3J"

func createBucket(cfg config.Config) error {
	creds := credentials.NewStaticV4(
		cfg.S3.AccessKeyID,
		cfg.S3.SecretAccessKey,
		"",
	)

	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: false,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.S3.BucketName)
	if err != nil {
		return err
	}

	if !exists {
		err = client.MakeBucket(
			ctx,
			cfg.S3.BucketName,
			minio.MakeBucketOptions{},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestBackup(t *testing.T) {
	// read the local development config file
	cfg, err := config.ReadFile("../../pg2s3.conf")
	if err != nil {
		t.Fatal(err)
	}

	err = createBucket(cfg)
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// generate name for backup
	name, err := pg2s3.GenerateBackupName(cfg.Backup.Prefix)
	if err != nil {
		t.Fatal(err)
	}

	// create backup
	backup, err := client.CreateBackup()
	if err != nil {
		t.Fatal(err)
	}

	// encrypt backup (if applicable)
	if len(cfg.Encryption.PublicKeys) > 0 {
		backup, err = client.EncryptBackup(backup, cfg.Encryption.PublicKeys)
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
	// read the local development config file
	cfg, err := config.ReadFile("../../pg2s3.conf")
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.NewClient(cfg)
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
	if len(cfg.Encryption.PublicKeys) > 0 {
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
	// read the local development config file
	cfg, err := config.ReadFile("../../pg2s3.conf")
	if err != nil {
		t.Fatal(err)
	}

	client, err := pg2s3.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		t.Fatal(err)
	}

	// delete all backups
	for _, backup := range backups {
		err = client.DeleteBackup(backup)
		if err != nil {
			t.Fatal(err)
		}
	}
}
