package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/jawher/mow.cli"
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

	backupsRetention := app.Int(cli.IntOpt{
		Name:   "backups-retention",
		Value:  30,
		Desc:   "The number of most recent backups that needs to be preserved",
		EnvVar: "BACKUPS_RETENTION",
	})

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
	log.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		envLevel := extractEnvironmentLevel(*pacEnvironment)
		log.Infof("System code: %s, App Name: %s, Environment level: %s, retention %v backups", *appSystemCode, *appName, envLevel, *backupsRetention)
		snapshotIDPrefix := pacAuroraPrefix + envLevel + "-backup"

		sess, err := session.NewSession()
		if err != nil {
			log.WithError(err).Error("Error in creating AWS session")
			return
		}
		svc := rds.New(sess)

		makeBackup(svc, envLevel, snapshotIDPrefix)
		backupCleanUp(svc, snapshotIDPrefix, *backupsRetention)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Error("App could not start")
		return
	}
}

func extractEnvironmentLevel(env string) string {
	firstHyphenIndex := strings.Index(env, "-")
	lastHyphenIndex := strings.LastIndex(env, "-")
	return env[firstHyphenIndex+1 : lastHyphenIndex]
}

func makeBackup(svc *rds.RDS, envLevel, snapshotIDPrefix string) {
	log.Info("Creating backup...")
	cluster, err := getDBCluster(svc, envLevel)
	if err != nil {
		log.WithError(err).Error("Error in fetching DB cluster information from AWS")
		return
	}

	snapshotID, err := makeDBSnapshots(svc, cluster, snapshotIDPrefix)
	if err != nil {
		log.WithError(err).Error("Error in creating DB snapshot")
		return
	}

	log.WithField("snapshotID", snapshotID).Info("PAC aurora backup successfully created")
}

func getDBCluster(svc *rds.RDS, envLevel string) (*rds.DBCluster, error) {
	clusterIdentifierPrefix := pacAuroraPrefix + envLevel
	isLastPage := false
	input := new(rds.DescribeDBClustersInput)
	for !isLastPage {
		result, err := svc.DescribeDBClusters(input)
		if err != nil {
			return nil, err
		}
		for _, cluster := range result.DBClusters {
			if strings.HasPrefix(*cluster.DBClusterIdentifier, clusterIdentifierPrefix) {
				return cluster, nil
			}
		}
		if result.Marker != nil {
			input.SetMarker(*result.Marker)
		} else {
			isLastPage = true
		}
	}
	return nil, fmt.Errorf("DB cluster not found with identifier prefix %v", clusterIdentifierPrefix)
}

func makeDBSnapshots(svc *rds.RDS, cluster *rds.DBCluster, snapshotIDPrefix string) (string, error) {
	input := new(rds.CreateDBClusterSnapshotInput)
	input.SetDBClusterIdentifier(*cluster.DBClusterIdentifier)
	timestamp := time.Now().Format("20060102")
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)

	_, err := svc.CreateDBClusterSnapshot(input)

	return snapshotIdentifier, err
}

func backupCleanUp(svc *rds.RDS, snapshotIDPrefix string, backupsRetention int) {
	log.Info("Cleaning up backups ...")
	_, err := getDBSnapshots(svc, snapshotIDPrefix)
	if err != nil {
		log.WithError(err).
			WithField("snapshotIDPrefix", snapshotIDPrefix).
			Error("Error in fetching DB cluster snapshots for backup cleanup")
		return
	}
}

func getDBSnapshots(svc *rds.RDS, snapshotIDPrefix string) ([]*rds.DBClusterSnapshot, error) {
	var snapshots []*rds.DBClusterSnapshot
	isLastPage := false
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetSnapshotType("manual")
	for !isLastPage {
		result, err := svc.DescribeDBClusterSnapshots(input)
		if err != nil {
			return nil, err
		}
		for _, snapshot := range result.DBClusterSnapshots {
			if strings.HasPrefix(*snapshot.DBClusterSnapshotIdentifier, snapshotIDPrefix) {
				snapshots = append(snapshots, snapshot)
			}
		}
		if result.Marker != nil {
			input.SetMarker(*result.Marker)
		} else {
			isLastPage = true
		}
	}
	return snapshots, nil
}
