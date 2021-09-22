package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/theandrew168/pg2s3"
	"golang.org/x/term"
)

// TODO: move env var names to package constants?

func main() {
	log.SetFlags(0)

	client, err := pg2s3.New(
		requireEnv("PG2S3_PG_CONNECTION_URI"),
		requireEnv("PG2S3_S3_ENDPOINT"),
		requireEnv("PG2S3_S3_ACCESS_KEY_ID"),
		requireEnv("PG2S3_S3_SECRET_ACCESS_KEY"),
		requireEnv("PG2S3_S3_BUCKET_NAME"))
	if err != nil {
		log.Fatalln(err)
	}

	prefix := requireEnv("PG2S3_BACKUP_PREFIX")
	retention, err := strconv.Atoi(requireEnv("PG2S3_BACKUP_RETENTION"))
	if err != nil {
		log.Fatalln(err)
	}

	usage := "usage: pg2s3 backup|restore|prune"
	if len(os.Args) < 2 {
		log.Fatalln(usage)
	}

	// TODO: verify connection to PG
	// TODO: verify connection to S3
	// TODO: verify age public key (if provided)

	cmd := os.Args[1]
	switch cmd {
	case "backup":
		err = backup(client, prefix)
		if err != nil {
			log.Fatalln(err)
		}
	case "restore":
		err = restore(client)
		if err != nil {
			log.Fatalln(err)
		}
	case "prune":
		err = prune(client, retention)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalln(usage)
	}
}

func requireEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("missing required env var: %s\n", name)
	}
	return value
}

func confirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/n]: ", message)

	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		return true
	} else {
		return false
	}
}

func backup(client *pg2s3.Client, prefix string) error {
	publicKey := os.Getenv("PG2S3_AGE_PUBLIC_KEY")

	// generate name for backup
	name, err := pg2s3.GenerateBackupName(prefix)
	if err != nil {
		return err
	}

	// generate path for backup
	path := pg2s3.GenerateBackupPath(name)

	// create backup
	err = client.CreateBackup(path)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	// encrypt backup (if applicable)
	if publicKey != "" {
		agePath := path + ".age"
		err := client.EncryptBackup(agePath, path, publicKey)
		if err != nil {
			return err
		}

		name = name + ".age"
		path = agePath
	}

	// upload backup
	err = client.UploadBackup(name, path)
	if err != nil {
		return err
	}

	log.Printf("created: %s\n", name)
	return nil
}

func restore(client *pg2s3.Client) error {
	publicKey := os.Getenv("PG2S3_AGE_PUBLIC_KEY")

	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		return errors.New("no backups present to restore")
	}

	// determine latest backup
	latest := backups[0]

	// generate path for backup
	path := pg2s3.GenerateBackupPath(latest)

	// download backup
	err = client.DownloadBackup(path, latest)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	// decrypt backup (if applicable)
	if publicKey != "" {
		fmt.Println("enter age private key:")
		input, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}

		privateKey := string(input)

		agePath := path
		path = strings.TrimSuffix(path, ".age")
		err = client.DecryptBackup(path, agePath, privateKey)
		if err != nil {
			return err
		}
	}

	// confirm restore before applying
	message := fmt.Sprintf("restore %s", latest)
	if !confirm(message) {
		return nil
	}

	// restore backup
	err = client.RestoreBackup(path)
	if err != nil {
		return err
	}
	log.Printf("restored: %s\n", latest)

	return nil
}

func prune(client *pg2s3.Client, retention int) error {
	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		return err
	}

	// check if backup count exceeds retention
	if len(backups) <= retention {
		return nil
	}

	// confirm deletion of all backups
	if retention < 1 {
		if !confirm("delete all backups") {
			return nil
		}
	}

	// determine expired backups to prune
	expired := backups[retention:]

	// prune old backups
	for _, backup := range expired {
		err = client.DeleteBackup(backup)
		if err != nil {
			return err
		}
		log.Printf("deleted: %s\n", backup)
	}

	return nil
}
