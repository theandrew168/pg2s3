package config_test

import (
	"testing"

	"github.com/theandrew168/pg2s3/internal/config"
)

func TestParseS3URL(t *testing.T) {
	s3URL := "s3://minioadmin:minioadmin@localhost:9000/pg2s3"

	s3, err := config.ParseS3URL(s3URL)
	if err != nil {
		t.Fatal(err)
	}

	if s3.Endpoint != "localhost:9000" {
		t.Errorf("got %q; want %q", s3.Endpoint, "localhost:9000")
	}
	if s3.AccessKeyID != "minioadmin" {
		t.Errorf("got %q; want %q", s3.AccessKeyID, "minioadmin")
	}
	if s3.SecretAccessKey != "minioadmin" {
		t.Errorf("got %q; want %q", s3.SecretAccessKey, "minioadmin")
	}
	if s3.BucketName != "pg2s3" {
		t.Errorf("got %q; want %q", s3.BucketName, "pg2s3")
	}
}
