# PAC Aurora backup

[![Circle CI](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/pac-aurora-backup/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/pac-aurora-backup)](https://goreportcard.com/report/github.com/Financial-Times/pac-aurora-backup) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/pac-aurora-backup/badge.svg)](https://coveralls.io/github/Financial-Times/pac-aurora-backup)

## Introduction

PAC Aurora backup is a command line tool for creating manual snapshots for an AWS Aurora cluster 
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
    
    export AWS_REGION=<aws-region> # e.g., eu-west-1
    export AWS_ACCESS_KEY_ID=<test-access-key-id> # available in lastpass note "AWS Keys for Snapshot"
    export AWS_SECRET_ACCESS_KEY=<test-secret-access-key> # available in lastpass note "AWS Keys for Snapshot"
   
    govendor test -v -race +local
    
    go install
    ```

2. Run the binary:

    ```
    $GOPATH/bin/pac-aurora-backup [--help]
  
    Options:                    
      --app-system-code         System Code of the application (env $APP_SYSTEM_CODE) (default "pac-aurora-backup")
      --app-name                Application name (env $APP_NAME) (default "pac-aurora-backup")
      --pac-environment         PAC environment (env $PAC_ENVIRONMENT)
      --aws-region              The AWS region of the Aurora cluster that needs a backup (env $AWS_REGION)
      --aws-access-key-id       The access key ID to access AWS (env $AWS_ACCESS_KEY_ID)
      --aws-secret-access-key   The secret access key to access AWS (env $AWS_SECRET_ACCESS_KEY)
    ```
    
    **NB: AWS access key ID and secret access key are available in lastpass note "AWS Keys for Snapshot"**
3. Test:

    By running the binary successfully you should find a new snapshot identified by the label 
    `pac-aurora-<enviroment-level>-backup-<date>` in the specified AWS region, 
    for instance `pac-aurora-staging-backup-20180112`.
   
## Build and deployment

* The application is built as a docker image inside a helm chart to be deployed as CronJob in a Kubernetes cluster.
  An internal Jenkins job takes care to push the Docker image to Docker Hub and deploy the chart when a tag is created.
  This is the Docker Hub repository: [coco/pac-aurora-backup](https://hub.docker.com/r/coco/pac-aurora-backup)
* CI provided by CircleCI: [pac-aurora-backup](https://circleci.com/gh/Financial-Times/pac-aurora-backup)

