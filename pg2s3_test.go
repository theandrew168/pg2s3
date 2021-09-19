package pg2s3_test

import (
	"strings"
	"testing"

	"github.com/theandrew168/pg2s3"
)

func TestBackup(t *testing.T) {

}

func TestRestore(t *testing.T) {

}

func TestPrune(t *testing.T) {

}

func TestGenerateBackupName(t *testing.T) {
	prefix := "pg2s3"
	suffix := ".backup"
	name := pg2s3.GenerateBackupName(prefix)

	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q is missing prefix %q", name, prefix)
	}
	if !strings.HasSuffix(name, suffix) {
		t.Errorf("name %q is missing suffix %q", name, suffix)
	}
}
