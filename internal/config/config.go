package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Backup struct {
	Prefix    string `toml:"prefix"`
	Retention int    `toml:"retention"`
	Schedule  string `toml:"schedule"`
}

type Restore struct {
	Schemas []string `toml:"schemas"`
}

type Encryption struct {
	PublicKeys []string `toml:"public_keys"`
}

type Config struct {
	S3 S3 `toml:"-"`

	PGURL      string     `toml:"pg_url"`
	S3URL      string     `toml:"s3_url"`
	Backup     Backup     `toml:"backup"`
	Restore    Restore    `toml:"restore"`
	Encryption Encryption `toml:"encryption"`
}

func Read(data string) (Config, error) {
	// init Config struct with default values
	cfg := Config{
		Backup: Backup{
			Prefix: "pg2s3",
		},
		Restore: Restore{
			Schemas: []string{},
		},
	}
	meta, err := toml.Decode(data, &cfg)
	if err != nil {
		return Config{}, err
	}

	// gather extra values
	var extra []string
	for _, keys := range meta.Undecoded() {
		key := keys[0]
		extra = append(extra, key)
	}

	// error upon extra values
	if len(extra) > 0 {
		msg := strings.Join(extra, ", ")
		return Config{}, fmt.Errorf("extra config values: %s", msg)
	}

	// build set of present top-level config keys
	present := make(map[string]bool)
	for _, keys := range meta.Keys() {
		key := keys[0]
		present[key] = true
	}

	required := []string{
		"pg_url",
		"s3_url",
	}

	// ensure required keys are present
	missing := []string{}
	for _, key := range required {
		if _, ok := present[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		msg := strings.Join(missing, ", ")
		return Config{}, fmt.Errorf("missing config values: %s", msg)
	}

	// parse S3 URL into S3 struct
	s3, err := ParseS3URL(cfg.S3URL)
	if err != nil {
		return Config{}, err
	}

	cfg.S3 = s3
	return cfg, nil
}

func ReadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	return Read(string(data))
}
