package main

import (
	"strings"
	"testing"
)

func TestGenerateBackupName(t *testing.T) {
	prefix := "pg2s3"
	suffix := ".backup"
	name := GenerateBackupName(prefix)

	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q is missing prefix %q", name, prefix)
	}
	if !strings.HasSuffix(name, suffix) {
		t.Errorf("name %q is missing suffix %q", name, suffix)
	}
}

//func TestGenerateBackupPath(t *testing.T) {
//	prefix := "pg2s3"
//	name := GenerateBackupName(prefix)
//	path := GenerateBackupPath(name)
//
//	pathDir := filepath.Dir(path)
//	tempDir := os.TempDir()
//	if pathDir != tempDir {
//		t.Errorf("path %q is not within temp dir %q", path, tempDir)
//	}
//}
