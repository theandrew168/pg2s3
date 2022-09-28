package pg2s3_test

import (
	"strings"
	"testing"
	"time"

	"github.com/theandrew168/pg2s3/internal/pg2s3"
)

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

	if !got.Equal(want) {
		t.Fatalf("want %v, got %v", want, got)
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
