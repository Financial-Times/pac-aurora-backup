package main

import (
	"os"
	"testing"
	"time"

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

	backupTime := time.Now()
	makeBackup(svc, testEnvironmentLevel, testSnapshotIDPrefix)

	assertCorrectBackup(t, testSnapshotIDPrefix, backupTime)
	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
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

func TestBackupCleanupSnapshotsHigherRetention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AWS RDS integration test")
	}

	totalSnapshots := 7
	backupsRetention := 4

	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)
	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	cluster, err := getDBClusterByEnvironmentLevel(svc, testEnvironmentLevel)
	require.NoError(t, err)

	var expectedSnapshotsIDs []string

	for i := 0; i < totalSnapshots; i++ {
		log.Infof("Waiting for cluster to to be ready for snapshot %v", i+1)
		waitForClusterToBeReady(t, svc, *cluster.DBClusterIdentifier)
		log.Infof("Creating snapshot %v", i+1)
		snapshotID, err := makeDBSnapshots(svc, cluster, testSnapshotIDPrefix)
		require.NoError(t, err)
		log.Infof("Waiting for snapshot %v to be ready", i+1)
		waitForSnapshotToBeReady(t, svc, snapshotID)
		if i >= totalSnapshots-backupsRetention {
			expectedSnapshotsIDs = append(expectedSnapshotsIDs, snapshotID)
		}
	}

	cleanUpBackup(svc, testSnapshotIDPrefix, backupsRetention)
	snapshots, err := getDBSnapshotsByPrefix(svc, testSnapshotIDPrefix)
	assert.NoError(t, err)
	assert.Len(t, snapshots, backupsRetention)
	for _, snapshot := range snapshots {
		assert.NoError(t, err)
		assert.Contains(t, expectedSnapshotsIDs, *snapshot.DBClusterSnapshotIdentifier)
	}

	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func TestBackupCleanupSnapshotsLowerRetention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AWS RDS integration test")
	}

	totalSnapshots := 3
	backupsRetention := 4

	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)
	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	cluster, err := getDBClusterByEnvironmentLevel(svc, testEnvironmentLevel)
	require.NoError(t, err)

	var expectedSnapshotsIDs []string

	for i := 0; i < totalSnapshots; i++ {
		log.Infof("Waiting for cluster to to be ready for snapshot %v", i+1)
		waitForClusterToBeReady(t, svc, *cluster.DBClusterIdentifier)
		log.Infof("Creating snapshot %v", i+1)
		snapshotID, err := makeDBSnapshots(svc, cluster, testSnapshotIDPrefix)
		require.NoError(t, err)
		log.Infof("Waiting for snapshot %v to be ready", i+1)
		waitForSnapshotToBeReady(t, svc, snapshotID)
		expectedSnapshotsIDs = append(expectedSnapshotsIDs, snapshotID)
	}

	cleanUpBackup(svc, testSnapshotIDPrefix, backupsRetention)
	snapshots, err := getDBSnapshotsByPrefix(svc, testSnapshotIDPrefix)
	assert.NoError(t, err)
	assert.Len(t, snapshots, totalSnapshots)
	for _, snapshot := range snapshots {
		assert.NoError(t, err)
		assert.Contains(t, expectedSnapshotsIDs, *snapshot.DBClusterSnapshotIdentifier)
	}

	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func assertCorrectBackup(t *testing.T, snapshotIDPrefix string, expectedBackupTime time.Time) {
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	snapshots, err := getDBSnapshotsByPrefix(svc, snapshotIDPrefix)
	assert.NoError(t, err)
	assert.Len(t, snapshots, 1)

	backupTimeLabel, err := time.Parse(snapshotIDPrefix+"-"+snapshotIDDateFormat, *snapshots[0].DBClusterSnapshotIdentifier)
	assert.NoError(t, err)
	assert.WithinDuration(t, expectedBackupTime, backupTimeLabel, 3*time.Second)
}

func cleanUpTestSnapshots(t *testing.T, snapshotIDPrefix string) {
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	snapshots, err := getDBSnapshotsByPrefix(svc, snapshotIDPrefix)
	require.NoError(t, err)

	for _, snapshot := range snapshots {
		waitForSnapshotToBeReady(t, svc, *snapshot.DBClusterSnapshotIdentifier)
		input := new(rds.DeleteDBClusterSnapshotInput)
		input.SetDBClusterSnapshotIdentifier(*snapshot.DBClusterSnapshotIdentifier)
		_, err = svc.DeleteDBClusterSnapshot(input)
		require.NoError(t, err)
	}
}

func waitForSnapshotToBeReady(t *testing.T, svc *rds.RDS, snapshotIdentifier string) {
	for i := 0; i < 20; i++ {
		input := new(rds.DescribeDBClusterSnapshotsInput)
		input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)
		result, err := svc.DescribeDBClusterSnapshots(input)
		require.NoError(t, err)

		require.Len(t, result.DBClusterSnapshots, 1)
		snapshot := result.DBClusterSnapshots[0]
		if *snapshot.Status == "available" || *snapshot.Status == "failed" {
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Fail()
}

func waitForClusterToBeReady(t *testing.T, svc *rds.RDS, clusterID string) {
	for i := 0; i < 20; i++ {
		input := new(rds.DescribeDBClustersInput)
		input.SetDBClusterIdentifier(clusterID)
		result, err := svc.DescribeDBClusters(input)
		require.NoError(t, err)

		require.Len(t, result.DBClusters, 1)
		cluster := result.DBClusters[0]
		if *cluster.Status == "available" {
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Fail()
}

func assertBackupNotExist(t *testing.T, snapshotIDPrefix string) {
	region, accessKeyID, secretAccessKey := getAWSAccessConfig(t)

	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	require.NoError(t, err)

	snapshots, err := getDBSnapshotsByPrefix(svc, snapshotIDPrefix)
	require.NoError(t, err)

	assert.Empty(t, snapshots)
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
