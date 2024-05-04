package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/go-co-op/gocron"
	"golang.org/x/term"

	"github.com/theandrew168/pg2s3/internal/config"
	"github.com/theandrew168/pg2s3/internal/pg2s3"
)

func main() {
	os.Exit(run())
}

func run() int {
	conf := flag.String("conf", "pg2s3.conf", "pg2s3 config file")
	flag.Parse()

	cfg, err := config.ReadFile(*conf)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	client, err := pg2s3.NewClient(cfg)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	// check for action (default run)
	args := flag.Args()
	var action string
	if len(args) == 0 {
		action = "run"
	} else {
		action = args[0]
	}

	// backup: create a new backup
	if action == "backup" {
		err = backup(client, cfg)
		if err != nil {
			fmt.Println(err)
			return 1
		}

		return 0
	}

	// restore: restore the most recent backup
	if action == "restore" {
		err = restore(client, cfg)
		if err != nil {
			fmt.Println(err)
			return 1
		}

		return 0
	}

	// prune: delete the oldest backups above the retention count
	if action == "prune" {
		err = prune(client, cfg)
		if err != nil {
			fmt.Println(err)
			return 1
		}

		return 0
	}

	if cfg.Backup.Schedule == "" {
		fmt.Println("no backup schedule specified, exiting")
		return 1
	}

	s := gocron.NewScheduler(time.UTC)
	s.Cron(cfg.Backup.Schedule).Do(func() {
		err := backup(client, cfg)
		if err != nil {
			fmt.Println(err)
			return
		}

		err = prune(client, cfg)
		if err != nil {
			fmt.Println(err)
			return
		}
	})

	// let systemd know that we are good to go (no-op if not using systemd)
	daemon.SdNotify(false, daemon.SdNotifyReady)
	fmt.Printf("running on schedule: %s\n", cfg.Backup.Schedule)

	s.StartBlocking()
	return 0
}

func confirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/n]: ", message)

	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		return true
	} else {
		return false
	}
}

func backup(client *pg2s3.Client, cfg config.Config) error {
	// generate name for backup
	name, err := pg2s3.GenerateBackupName(cfg.Backup.Prefix)
	if err != nil {
		return err
	}

	// create backup
	backup, err := client.CreateBackup()
	if err != nil {
		return err
	}

	// encrypt backup (if applicable)
	if len(cfg.Encryption.PublicKeys) > 0 {
		backup, err = client.EncryptBackup(backup, cfg.Encryption.PublicKeys)
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

	fmt.Printf("created %s\n", name)
	return nil
}

func restore(client *pg2s3.Client, cfg config.Config) error {
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
	if len(cfg.Encryption.PublicKeys) > 0 {
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
	fmt.Printf("restored %s\n", latest)

	return nil
}

func prune(client *pg2s3.Client, cfg config.Config) error {
	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		return err
	}

	// exit early if retention is zero or limit hasn't been reached
	if cfg.Backup.Retention <= 0 || len(backups) <= cfg.Backup.Retention {
		return nil
	}

	// determine expired backups to prune
	expired := backups[cfg.Backup.Retention:]

	// prune old backups
	for _, backup := range expired {
		err = client.DeleteBackup(backup)
		if err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", backup)
	}

	return nil
}
