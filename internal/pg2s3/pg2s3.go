package pg2s3

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

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
