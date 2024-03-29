package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Financial-Times/pac-aurora-backup/backup"
	cli "github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
)

const pacAuroraPrefix = "pac-aurora-"

func main() {
	app := cli.App("pac-aurora-backup", "A backup app for PAC Aurora clusters")

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "pac-aurora-backup",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "pac-aurora-backup",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	pacEnvironment := app.String(cli.StringOpt{
		Name:   "pac-environment",
		Desc:   "PAC environment",
		EnvVar: "PAC_ENVIRONMENT",
	})

	rdsRegion := app.String(cli.StringOpt{
		Name:   "rds-region",
		Desc:   "The AWS region of the Aurora cluster that needs a backup",
		EnvVar: "RDS_REGION",
	})

	backupsRetention := app.Int(cli.IntOpt{
		Name:   "backups-retention",
		Value:  35,
		Desc:   "The number of most recent backups that needed to be preserved",
		EnvVar: "BACKUPS_RETENTION",
	})

	statusCheckIntervalString := app.String(cli.StringOpt{
		Name:   "status-check-interval",
		Value:  "30s",
		Desc:   "The time elapsed between each check of a status for AWS RDS resources",
		EnvVar: "STATUS_CHECK_INTERVAL",
	})

	statusCheckAttempts := app.Int(cli.IntOpt{
		Name:   "status-check-attempts",
		Value:  60,
		Desc:   "The number of attempts to check of a status for AWS RDS resources",
		EnvVar: "STATUS_CHECK_ATTEMPTS",
	})

	log.SetFormatter(&log.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	log.SetLevel(log.InfoLevel)

	log.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {

		log.Infof("System code: %s, App Name: %s, Pac environment: %s", *appSystemCode, *appName, *pacEnvironment)

		statusCheckInterval, err := time.ParseDuration(*statusCheckIntervalString)
		if err != nil {
			log.WithError(err).Warn("Error in parsing status-check-interval parameter. Setting the value as 30s")
			statusCheckInterval = 30 * time.Second
		}

		envLevel, err := extractEnvironmentLevel(*pacEnvironment)
		if err != nil {
			log.WithError(err).Error("Error in extracting environment level")
			return
		}

		clusterIDPrefix := pacAuroraPrefix + envLevel
		snapshotIDPrefix := clusterIDPrefix + "-backup"

		svc, err := backup.NewBackupService(*rdsRegion, clusterIDPrefix, snapshotIDPrefix, statusCheckInterval, *statusCheckAttempts, *backupsRetention)
		if err != nil {
			log.WithError(err).Error("Error in creating a new backup service")
			return
		}

		svc.MakeBackup()
		svc.CleanUpOldBackups()

	}

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Error("App could not start")
		return
	}
	log.Infof("[Shutdown] %v is stopping", *appSystemCode)
}

func extractEnvironmentLevel(env string) (string, error) {
	firstHyphenIndex := strings.Index(env, "-")
	lastHyphenIndex := strings.LastIndex(env, "-")
	if firstHyphenIndex == lastHyphenIndex {
		return "", fmt.Errorf("environment label is invalid: %v", env)
	}
	envLevel := env[firstHyphenIndex+1 : lastHyphenIndex]
	if envLevel == "" {
		return "", fmt.Errorf("environment label is invalid: %v", env)
	}
	return envLevel, nil
}
