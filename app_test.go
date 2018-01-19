package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	testLog "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEnvironmentLevel = "staging"
const testSnapshotIDPrefix = pacAuroraPrefix + testEnvironmentLevel + "-test-backup"

func TestExtractEnvironmentLevel(t *testing.T) {
	env1 := "pac-staging-eu"
	envLevel1, err := extractEnvironmentLevel(env1)
	assert.NoError(t, err)
	assert.Equal(t, "staging", envLevel1)

	env2 := "pac-prod-us"
	envLevel2, err := extractEnvironmentLevel(env2)
	assert.NoError(t, err)
	assert.Equal(t, "prod", envLevel2)
}

func TestExtractEnvironmentLevelError(t *testing.T) {
	_, err := extractEnvironmentLevel("pacstagingeu")
	assert.Error(t, err)

	_, err = extractEnvironmentLevel("pac-us")
	assert.Error(t, err)

	_, err = extractEnvironmentLevel("pac--us")
	assert.Error(t, err)
}

func TestHappyBackup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AWS RDS integration test")
	}
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)
	makeBackup(svc, testEnvironmentLevel, testSnapshotIDPrefix)

	assertCorrectBackup(t, testEnvironmentLevel, testSnapshotIDPrefix)
	cleanUpSnapshot(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueMissingDBCluster(t *testing.T) {
	hook := testLog.NewGlobal()

	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)
	makeBackup(svc, testEnvironmentLevel+"-that-does-not-exist", testSnapshotIDPrefix)

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in fetching DB cluster information from AWS", hook.LastEntry().Message)
	assert.EqualError(t, hook.LastEntry().Data["error"].(error), "DB cluster not found with identifier prefix pac-aurora-staging-that-does-not-exist")

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueDBClusterCreationError(t *testing.T) {
	hook := testLog.NewGlobal()

	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)
	makeBackup(svc, testEnvironmentLevel, "")

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in creating DB snapshot", hook.LastEntry().Message)

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func assertCorrectBackup(t *testing.T, env string, snapshotIDPrefix string) {
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

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
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

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
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	timestamp := time.Now().Format("20060102")
	snapshotIdentifier := snapshotIDPrefix + "-" + timestamp
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
	_, err = svc.DescribeDBClusterSnapshots(input)
	assert.Equal(t, err.(awserr.Error).Code(), rds.ErrCodeDBClusterSnapshotNotFoundFault)
}

func getAWSAccessConfig(t *testing.T) (region, accessKeyID, secretAccessKey string) {
	region = os.Getenv("AWS_REGION")
	require.NotEmpty(t, region, "You need to set AWS_REGION environment variable if want to run this test")
	accessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	require.NotEmpty(t, region, "You need to set AWS_ACCESS_KEY_ID environment variable if want to run this test")
	secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	require.NotEmpty(t, region, "You need to set AWS_SECRET_ACCESS_KEY environment variable if want to run this test")
	return
}
