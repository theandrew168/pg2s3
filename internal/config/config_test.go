package config_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/theandrew168/pg2s3/internal/config"
)

const (
	pgURL = "postgresql://postgres:postgres@localhost:5432/postgres"
	s3URL = "s3://minioadmin:minioadmin@localhost:9000/pg2s3"
)

func TestRead(t *testing.T) {
	data := fmt.Sprintf(`
		pg_url = "%s"
		s3_url = "%s"
		
		[backup]
		prefix = "foobar"
		retention = 30
		schedule = "0 9 * * *"

		[restore]
		schemas = ["foo", "bar"]
		
		[encryption]
		public_keys = [
			"age156hm5jvxfvf8xf0zjs52gc5hhq64rt23gw3fehqj2vu77sk07a5qvplj52",
		]
	`, pgURL, s3URL)

	cfg, err := config.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PGURL != pgURL {
		t.Errorf("got %q; want %q", cfg.PGURL, pgURL)
	}
	if cfg.S3URL != s3URL {
		t.Errorf("got %q; want %q", cfg.S3URL, s3URL)
	}
	if cfg.Backup.Prefix != "foobar" {
		t.Errorf("got %q; want %q", cfg.Backup.Prefix, "foobar")
	}
	if cfg.Backup.Retention != 30 {
		t.Errorf("got %v; want %v", cfg.Backup.Retention, 30)
	}
	if cfg.Backup.Schedule != "0 9 * * *" {
		t.Errorf("got %q; want %q", cfg.Backup.Schedule, "0 9 * * *")
	}
	if !reflect.DeepEqual(cfg.Restore.Schemas, []string{"foo", "bar"}) {
		t.Errorf("got %v; want %v", cfg.Restore.Schemas, []string{"foo", "bar"})
	}
	if !reflect.DeepEqual(
		cfg.Encryption.PublicKeys,
		[]string{"age156hm5jvxfvf8xf0zjs52gc5hhq64rt23gw3fehqj2vu77sk07a5qvplj52"},
	) {
		t.Errorf(
			"got %q; want %q",
			cfg.Encryption.PublicKeys,
			[]string{"age156hm5jvxfvf8xf0zjs52gc5hhq64rt23gw3fehqj2vu77sk07a5qvplj52"},
		)
	}
}

func TestOptional(t *testing.T) {
	data := fmt.Sprintf(`
		pg_url = "%s"
		s3_url = "%s"
	`, pgURL, s3URL)

	cfg, err := config.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PGURL != pgURL {
		t.Errorf("got %q; want %q", cfg.PGURL, pgURL)
	}
	if cfg.S3URL != s3URL {
		t.Errorf("got %q; want %q", cfg.S3URL, s3URL)
	}
	if cfg.Backup.Prefix != "pg2s3" {
		t.Errorf("got %q; want %q", cfg.Backup.Prefix, "pg2s3")
	}
	if !reflect.DeepEqual(cfg.Restore.Schemas, []string{}) {
		t.Errorf("got %v; want %v", cfg.Restore.Schemas, []string{})
	}
}

func TestRequired(t *testing.T) {
	data := `
		[backup]
		prefix = "foobar"
		retention = 30
		schedule = "0 9 * * *"
	`

	_, err := config.Read(data)
	if err == nil {
		t.Fatalf("got: nil; want: error")
	}

	if !strings.Contains(err.Error(), "pg_url") {
		t.Errorf("got %q; want to contain: %q", err.Error(), "pg_url")
	}
	if !strings.Contains(err.Error(), "s3_url") {
		t.Errorf("got %q; want to contain: %q", err.Error(), "s3_url")
	}
}

func TestExtra(t *testing.T) {
	data := `
		foo = "bar"

		[backup]
		prefix = "foobar"
		retention = 30
		schedule = "0 9 * * *"
	`

	_, err := config.Read(data)
	if err == nil {
		t.Fatalf("got: nil; want: error")
	}

	if !strings.Contains(err.Error(), "extra") {
		t.Errorf("got %q; want to contain: %q", err.Error(), "extra")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("got %q; want to contain: %q", err.Error(), "foo")
	}
}
