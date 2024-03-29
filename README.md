# PAC Aurora backup

[![Circle CI](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/pac-aurora-backup)](https://goreportcard.com/report/github.com/Financial-Times/pac-aurora-backup) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/pac-aurora-backup/badge.svg)](https://coveralls.io/github/Financial-Times/pac-aurora-backup)

## Introduction

PAC Aurora backup is a command line tool for creating manual snapshots for an AWS Aurora cluster 
that belongs to PAC. 

## Installation

Download the source code and build the project:

```shell
git clone https://github.com/Financial-Times/pac-aurora-backup.git
cd pac-aurora-backup
go build -mod=readonly .
```

### Run

```shell
./pac-aurora-backup [--help]

Options:
  --app-system-code         System Code of the application (env $APP_SYSTEM_CODE) (default "pac-aurora-backup")
  --app-name                Application name (env $APP_NAME) (default "pac-aurora-backup")
  --pac-environment         PAC environment (env $PAC_ENVIRONMENT)
  --rds-region              The AWS region of the Aurora cluster that needs a backup (env $RDS_REGION)
  --backups-retention       The number of most recent backups that needed to be preserved (env $BACKUPS_RETENTION) (default 35)
  --status-check-interval   The time elapsed between each check of a status for AWS RDS resources (env $STATUS_CHECK_INTERVAL) (default "30s")
  --status-check-attempts   The number of attempts to check of a status for AWS RDS resources (env $STATUS_CHECK_ATTEMPTS) (default 60)
```

#### Running in Kubernetes

The app is using ServiceAccount which is linked to AWS IAM Role, as a result upon pod creation AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE envvars are being injected into the pod and the aws-sdk-go uses them behind the scenes.

## Test

### Unit tests

```shell
go test -v -race ./...
```

### Integration tests

 ```shell
export RUN_BACKUP_TESTS=1 # enable integration tests
export AWS_REGION=<aws-region> # e.g., eu-west-1
export AWS_ACCESS_KEY_ID=<test-access-key-id> # available in lastpass note "AWS Keys for Snapshot"
export AWS_SECRET_ACCESS_KEY=<test-secret-access-key> # available in lastpass note "AWS Keys for Snapshot"
export AWS_SESSION_TOKEN=<session-token> # if you are using temporary credentials
go test -v -race ./...
```

By running the binary successfully you should find a new snapshot identified by the label
`pac-aurora-<enviroment-level>-backup-<date>` in the specified AWS region,
for instance `pac-aurora-staging-backup-20180112`.

## Build and deployment

* The application is built as a Docker image inside a Helm chart to be deployed as cronjob in a Kubernetes cluster.
  An internal Jenkins job takes care to push the Docker image to DockerHub and deploy the chart when a tag is created.
  This is the DockerHub repository: [coco/pac-aurora-backup](https://hub.docker.com/r/coco/pac-aurora-backup)
* CI provided by CircleCI: [pac-aurora-backup](https://circleci.com/gh/Financial-Times/pac-aurora-backup)

## Recommendations based on AWS limits

The app and its unit tests can genuinely fail due to some aspects on how AWS RDS service is designed.
The conditions for this app and its tests to run properly are:
 * the source DB cluster needs to be in status `available`;
 * no other snapshots are in `creation` state for the source DB cluster.
 
A safe way to make sure that the conditions above are respected is avoiding to run this app 
in the following scenarios:
 * during the RDS automatic backup time window (see details [here](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_WorkingWithAutomatedBackups.html#USER_WorkingWithAutomatedBackups.BackupWindow));
 * during the RDS maintenance time window (see details [here](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_UpgradeDBInstance.Maintenance.html#Concepts.DBMaintenance));
 * running this app in parallel for the same DB cluster.
 
 
 
