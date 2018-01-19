package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
)

const pacAuroraPrefix = "pac-aurora-"
const snapshotIDDateFormat = "2006-01-02-15-04-05"

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

	awsRegion := app.String(cli.StringOpt{
		Name:   "aws-region",
		Desc:   "The AWS region of the Aurora cluster that needs a backup",
		EnvVar: "AWS_REGION",
	})

	awsAccessKeyID := app.String(cli.StringOpt{
		Name:   "aws-access-key-id",
		Desc:   "The access key ID to access AWS",
		EnvVar: "AWS_ACCESS_KEY_ID",
	})

	awsSecretAccessKey := app.String(cli.StringOpt{
		Name:   "aws-secret-access-key",
		Desc:   "The secret access key to access AWS",
		EnvVar: "AWS_SECRET_ACCESS_KEY",
	})

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
	log.Infof("[Startup] %v is starting", *appSystemCode)

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Pac environment: %s", *appSystemCode, *appName, *pacEnvironment)
		envLevel, err := extractEnvironmentLevel(*pacEnvironment)
		if err != nil {
			log.WithError(err).Error("Error in extracting environment level")
			return
		}

		snapshotIDPrefix := pacAuroraPrefix + envLevel + "-backup"

		svc, err := newRDSService(*awsRegion, *awsAccessKeyID, *awsSecretAccessKey)
		if err != nil {
			log.WithError(err).Error("Error in connecting to AWS RDS")
			return
		}

		makeBackup(svc, envLevel, snapshotIDPrefix)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Error("App could not start")
		return
	}
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

func newRDSService(region, accessKeyID, secretAccessKey string) (*rds.RDS, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
	})
	if err != nil {
		return nil, err
	}
	return rds.New(sess), nil
}

func makeBackup(svc *rds.RDS, envLevel, snapshotIDPrefix string) {
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
	timestamp := time.Now().Format(snapshotIDDateFormat)
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)

	_, err := svc.CreateDBClusterSnapshot(input)

	return snapshotIdentifier, err
}
