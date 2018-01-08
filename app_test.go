package main

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	testLog "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEnvironment = "staging"
const testSnapshotIDPrefix = pacAuroraPrefix + testEnvironment + "-test-backup"

func TestHappyBackup(t *testing.T) {
	makeBackup(testEnvironment, testSnapshotIDPrefix)

	assertCorrectBackup(t, testEnvironment, testSnapshotIDPrefix)
	cleanUpSnapshot(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueMissingDBCluster(t *testing.T) {
	hook := testLog.NewGlobal()

	makeBackup(testEnvironment+"-that-does-not-exist", testSnapshotIDPrefix)

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in fetching DB cluster information from AWS", hook.LastEntry().Message)
	assert.EqualError(t, hook.LastEntry().Data["error"].(error), "DB cluster not found with identifier prefix pac-aurora-staging-that-does-not-exist")

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueDBClusterCreationError(t *testing.T) {
	hook := testLog.NewGlobal()

	makeBackup(testEnvironment, "")

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in creating DB snapshot", hook.LastEntry().Message)

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func assertCorrectBackup(t *testing.T, env string, snapshotIDPrefix string) {
	sess, err := session.NewSession()
	require.NoError(t, err)
	svc := rds.New(sess)

	timestamp := time.Now().Format("20060102")
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
	result, err := svc.DescribeDBClusterSnapshots(input)

	assert.NoError(t, err)
	assert.Len(t, result.DBClusterSnapshots, 1)
	snapshot := result.DBClusterSnapshots[0]
	assert.True(t, strings.HasPrefix(*snapshot.DBClusterIdentifier, pacAuroraPrefix+env))
}

func cleanUpSnapshot(t *testing.T, snapshotIDPrefix string) {
	sess, err := session.NewSession()
	require.NoError(t, err)
	svc := rds.New(sess)

	timestamp := time.Now().Format("20060102")
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp

	waitForSnapshotToBeReady(t, svc, snapshotIdentifier)

	input := new(rds.DeleteDBClusterSnapshotInput)
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
	_, err = svc.DeleteDBClusterSnapshot(input)
	require.NoError(t, err)
}

func waitForSnapshotToBeReady(t *testing.T, svc *rds.RDS, snapshotIdentifier string) {
	for {
		input := new(rds.DescribeDBClusterSnapshotsInput)
		input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
		result, err := svc.DescribeDBClusterSnapshots(input)
		require.NoError(t, err)

		require.Len(t, result.DBClusterSnapshots, 1)
		snapshot := result.DBClusterSnapshots[0]
		if *snapshot.Status == "available" || *snapshot.Status == "failed" {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func assertBackupNotExist(t *testing.T, snapshotIDPrefix string) {
	sess, err := session.NewSession()
	require.NoError(t, err)
	svc := rds.New(sess)

	timestamp := time.Now().Format("20060102")
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
	_, err = svc.DescribeDBClusterSnapshots(input)
	assert.Equal(t, err.(awserr.Error).Code(), rds.ErrCodeDBClusterSnapshotNotFoundFault)
}
