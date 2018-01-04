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

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
	log.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s", *appSystemCode, *appName)
		sess, err := session.NewSession()
		if err != nil {
			log.WithError(err).Error("Error in creating AWS session")
			return
		}
		svc := rds.New(sess)

		cluster, err := getDBCluster(svc, *pacEnvironment)
		if err != nil {
			log.WithError(err).Error("Error in fetching DB cluster information from AWS")
		}

		err = makeDBSnapshots(svc, cluster, *pacEnvironment)
		if err != nil {
			log.WithError(err).Error("Error in creating DB snapshot")
		}
	}

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Error("App could not start")
		return
	}

}

func getDBCluster(svc *rds.RDS, pacEnvironment string) (*rds.DBCluster, error) {
	clusterIdentifierPrefix := pacAuroraPrefix + pacEnvironment
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
			input.Marker = result.Marker
		} else {
			isLastPage = true
		}
	}
	return nil, fmt.Errorf("DB cluster not found with identifier prefix %v", clusterIdentifierPrefix)
}

func makeDBSnapshots(svc *rds.RDS, cluster *rds.DBCluster, pacEnvironment string) error {
	input := new(rds.CreateDBClusterSnapshotInput)
	input.SetDBClusterIdentifier(*cluster.DBClusterIdentifier)
	timestamp := time.Now().Format("20160102")
	snapshotIdentifier := pacAuroraPrefix + pacEnvironment + "-backup-" + timestamp
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)

	_, err := svc.CreateDBClusterSnapshot(input)
	return err
}
