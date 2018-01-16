# PAC Aurora backup

[![Circle CI](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/pac-aurora-backup)](https://goreportcard.com/report/github.com/Financial-Times/pac-aurora-backup) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/pac-aurora-backup/badge.svg)](https://coveralls.io/github/Financial-Times/pac-aurora-backup)

## Introduction

PAC Aurora backup is a command line tool for creating manual snapshots for a AWS aurora cluster 
that belongs to PAC. 

## Installation

Download the source code, dependencies and test dependencies:

```
go get -u github.com/kardianos/govendor
mkdir $GOPATH/src/github.com/Financial-Times/pac-aurora-backup
cd $GOPATH/src/github.com/Financial-Times
git clone https://github.com/Financial-Times/pac-aurora-backup.git
cd pac-aurora-backup && govendor sync
go build .
```

## Running locally

1. Run the tests and install the binary:

    ```
    govendor sync
    govendor test -v -race +local
    go install
    ```

2. Run the binary:

    ```
    export AWS_REGION=<aws-region> # e.g., eu-west-1
    export AWS_ACCESS_KEY_ID=<access-key-id> # available in lastpass note "AWS Keys for Snapshot"
    export AWS_SECRET_ACCESS_KEY=<secret-access-key> # available in lastpass note "AWS Keys for Snapshot"
    export PAC_ENVIRONMENT=<pac-environment> # e.g., pac-staging-eu

    $GOPATH/bin/pac-aurora-backup 
    ```
3. Test:

    By running the binary successfully you should find a new snapshot identified by the label 
    `pac-aurora-<enviroment-level>-backup-<date>` in the specified AWS region, 
    for instance `pac-aurora-staging-backup-20180112`.
   
## Build and deployment

* The application is built as a docker image inside a helm chart to be deployed as CronJob in a Kubernetes cluster.
  An internal Jenkins job takes care to push the Docker image to Docker Hub and deploy the chart when a tag is created.
  This is the Docker Hub repository: [coco/pac-aurora-backup](https://hub.docker.com/r/coco/pac-aurora-backup)
* CI provided by CircleCI: [pac-aurora-backup](https://circleci.com/gh/Financial-Times/pac-aurora-backup)

