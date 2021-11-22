package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"golang.org/x/term"

	"github.com/theandrew168/pg2s3"
)

func main() {
	log.SetFlags(0)

	conf := flag.String("conf", "pg2s3.conf", "pg2s3 config file")
	actionBackup := flag.Bool("backup", false, "pg2s3 action: backup")
	actionRestore := flag.Bool("restore", false, "pg2s3 action: restore")
	actionPrune := flag.Bool("prune", false, "pg2s3 action: prune")
	flag.Parse()

	cfg, err := pg2s3.ReadConfig(*conf)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := pg2s3.New(cfg)
	if err != nil {
		log.Fatalln(err)
	}

	// check how many actions were specified
	count := 0
	actions := []bool{*actionBackup, *actionRestore, *actionPrune}
	for _, action := range actions {
		if action {
			count++
		}
	}

	if *actionBackup {
		err = backup(client, cfg)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if *actionRestore {
		err = restore(client, cfg)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if *actionPrune {
		err = prune(client, cfg)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if count == 0 && cfg.BackupSchedule != "" {
		log.Println("running scheduler")

		s := gocron.NewScheduler(time.UTC)
		s.Cron(cfg.BackupSchedule).Do(func(){
			err := backup(client, cfg)
			if err != nil {
				log.Println(err)
				return
			}

			err = prune(client, cfg)
			if err != nil {
				log.Println(err)
				return
			}
		})
		s.StartBlocking()
	}
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

func backup(client *pg2s3.Client, cfg pg2s3.Config) error {
	// generate name for backup
	name, err := pg2s3.GenerateBackupName(cfg.BackupPrefix)
	if err != nil {
		return err
	}

	// create backup
	backup, err := client.CreateBackup()
	if err != nil {
		return err
	}

	// encrypt backup (if applicable)
	if cfg.AgePublicKey != "" {
		backup, err = client.EncryptBackup(backup, cfg.AgePublicKey)
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

func restore(client *pg2s3.Client, cfg pg2s3.Config) error {
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
	if cfg.AgePublicKey != "" {
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

func prune(client *pg2s3.Client, cfg pg2s3.Config) error {
	// list all backups
	backups, err := client.ListBackups()
	if err != nil {
		return err
	}

	// exit early if retention is zero or limit hasn't been reached
	if cfg.BackupRetention <= 0 || len(backups) <= cfg.BackupRetention {
		return nil
	}

	// determine expired backups to prune
	expired := backups[cfg.BackupRetention:]

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
