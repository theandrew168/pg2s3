package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/go-co-op/gocron/v2"
	"golang.org/x/term"

	"github.com/theandrew168/pg2s3/internal/config"
	"github.com/theandrew168/pg2s3/internal/pg2s3"
)

func main() {
	code := 0

	err := run()
	if err != nil {
		fmt.Println(err)
		code = 1
	}

	os.Exit(code)
}

func run() error {
	conf := flag.String("conf", "pg2s3.conf", "pg2s3 config file")
	flag.Parse()

	cfg, err := config.ReadFile(*conf)
	if err != nil {
		return err
	}

	client, err := pg2s3.NewClient(cfg)
	if err != nil {
		return err
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
		return backup(client, cfg)
	}

	// restore: restore the most recent backup
	if action == "restore" {
		return restore(client, cfg)
	}

	// prune: delete the oldest backups above the retention count
	if action == "prune" {
		return prune(client, cfg)
	}

	if cfg.Backup.Schedule == "" {
		// TODO: replace with logging
		fmt.Println("no backup schedule specified, exiting")
		return nil
	}

	s, err := gocron.NewScheduler(
		gocron.WithLocation(time.UTC),
	)
	if err != nil {
		return err
	}
	_, err = s.NewJob(
		gocron.CronJob(cfg.Backup.Schedule, false),
		gocron.NewTask(func() {
			err := backup(client, cfg)
			if err != nil {
				// TODO: replace with logging
				fmt.Println(err)
				return
			}

			err = prune(client, cfg)
			if err != nil {
				// TODO: replace with logging
				fmt.Println(err)
				return
			}
		}),
	)
	if err != nil {
		return err
	}

	// let systemd know that we are good to go (no-op if not using systemd)
	daemon.SdNotify(false, daemon.SdNotifyReady)
	// TODO: replace with logging
	fmt.Printf("running on schedule: %s\n", cfg.Backup.Schedule)

	// create a context that cancels upon receiving an interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	s.Start()

	<-ctx.Done()
	err = s.Shutdown()
	if err != nil {
		return err
	}

	return nil
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
