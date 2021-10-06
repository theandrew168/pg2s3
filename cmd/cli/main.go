package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"filippo.io/age"
	"github.com/theandrew168/pg2s3"
	"golang.org/x/term"
)

func main() {
	log.SetFlags(0)

	usage := "usage: pg2s3 backup|restore|prune"
	if len(os.Args) < 2 {
		log.Fatalln(usage)
	}

	client, err := pg2s3.New(
		requireEnv(pg2s3.EnvPGConnectionURI),
		requireEnv(pg2s3.EnvS3Endpoint),
		requireEnv(pg2s3.EnvS3AccessKeyID),
		requireEnv(pg2s3.EnvS3SecretAccessKey),
		requireEnv(pg2s3.EnvS3BucketName))
	if err != nil {
		log.Fatalln(err)
	}

	prefix := requireEnv(pg2s3.EnvBackupPrefix)
	retention, err := strconv.Atoi(requireEnv(pg2s3.EnvBackupRetention))
	if err != nil {
		log.Fatalln(err)
	}

	// validate public key (if provided)
	publicKey := os.Getenv(pg2s3.EnvAgePublicKey)
	if publicKey != "" {
		_, err = age.ParseX25519Recipient(publicKey)
		if err != nil {
			log.Fatalln(err)
		}
	}

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
	publicKey := os.Getenv(pg2s3.EnvAgePublicKey)

	// generate name for backup
	name, err := pg2s3.GenerateBackupName(prefix)
	if err != nil {
		return err
	}

	// create backup
	backup, err := client.CreateBackup()
	if err != nil {
		return err
	}

	// encrypt backup (if applicable)
	if publicKey != "" {
		backup, err = client.EncryptBackup(backup, publicKey)
		if err != nil {
			return err
		}

		name = name + ".age"
	}

	// upload backup
	err = client.UploadBackup(name, backup)
	if err != nil {
		return err
	}

	log.Printf("created %s\n", name)
	return nil
}

func restore(client *pg2s3.Client) error {
	publicKey := os.Getenv(pg2s3.EnvAgePublicKey)

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

	// download backup
	backup, err := client.DownloadBackup(latest)
	if err != nil {
		return err
	}

	// decrypt backup (if applicable)
	if publicKey != "" {
		fmt.Print("enter private key: ")
		input, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}

		fmt.Println()
		privateKey := string(input)

		backup, err = client.DecryptBackup(backup, privateKey)
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
	err = client.RestoreBackup(backup)
	if err != nil {
		return err
	}
	log.Printf("restored %s\n", latest)

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
		log.Printf("deleted %s\n", backup)
	}

	return nil
}
