package backup

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

const testClusterIDPrefix = "pac-aurora-staging"
const testSnapshotIDPrefix = testClusterIDPrefix + "-test-backup"
const testStatusCheckInterval = 15 * time.Second
const testStatusCheckAttempts = 60

func TestHappyBackup(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}
	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix, testSnapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	backupTime := time.Now().UTC()
	svc.MakeBackup()

	assertCorrectBackup(t, testSnapshotIDPrefix, backupTime)
	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueMissingDBCluster(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	hook := testLog.NewGlobal()

	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix+"-that-does-not-exist", testSnapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	svc.MakeBackup()

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in fetching DB cluster information from AWS", hook.LastEntry().Message)
	assert.EqualError(t, hook.LastEntry().Data["error"].(error), "DB cluster not found with identifier prefix pac-aurora-staging-that-does-not-exist")

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func TestUnhappyBackupDueDBClusterCreationError(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	hook := testLog.NewGlobal()

	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix, "", testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	svc.MakeBackup()

	assert.Equal(t, log.ErrorLevel, hook.LastEntry().Level)
	assert.Equal(t, "Error in creating DB snapshot", hook.LastEntry().Message)

	assertBackupNotExist(t, testSnapshotIDPrefix)
}

func TestBackupCleanupSnapshotsHigherRetention(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	region := getAWSAccessConfig(t)

	totalSnapshots := 7
	backupsRetention := 4

	svc, err := NewBackupService(region, testClusterIDPrefix, testSnapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, backupsRetention)

	require.NoError(t, err)

	clusterID, err := svc.(*auroraBackupService).getDBClusterID()
	require.NoError(t, err)

	var expectedSnapshotsIDs []string

	for i := 0; i < totalSnapshots; i++ {
		log.Infof("Waiting for cluster to to be ready for snapshot %v", i+1)
		waitForClusterToBeReady(t, svc.(*auroraBackupService).RDS, clusterID)
		log.Infof("Creating snapshot %v", i+1)
		snapshotID, err := svc.(*auroraBackupService).makeDBSnapshots(clusterID)
		require.NoError(t, err)
		log.Infof("Waiting for snapshot %v to be ready", i+1)
		err = svc.(*auroraBackupService).checkSnapshotCreation(snapshotID)
		require.NoError(t, err)
		if i >= totalSnapshots-backupsRetention {
			expectedSnapshotsIDs = append(expectedSnapshotsIDs, snapshotID)
		}
	}

	svc.CleanUpOldBackups()
	snapshots, err := svc.(*auroraBackupService).getDBSnapshotsByPrefix()
	assert.NoError(t, err)
	assert.Len(t, snapshots, backupsRetention)
	for _, snapshot := range snapshots {
		assert.NoError(t, err)
		assert.Contains(t, expectedSnapshotsIDs, *snapshot.DBClusterSnapshotIdentifier)
	}

	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func TestBackupCleanupSnapshotsLowerRetention(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	totalSnapshots := 3
	backupsRetention := 4

	region := getAWSAccessConfig(t)
	svc, err := NewBackupService(region, testClusterIDPrefix, testSnapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, backupsRetention)
	require.NoError(t, err)

	clusterID, err := svc.(*auroraBackupService).getDBClusterID()
	require.NoError(t, err)

	var expectedSnapshotsIDs []string

	for i := 0; i < totalSnapshots; i++ {
		log.Infof("Waiting for cluster to to be ready for snapshot %v", i+1)
		waitForClusterToBeReady(t, svc.(*auroraBackupService).RDS, clusterID)
		log.Infof("Creating snapshot %v", i+1)
		snapshotID, err := svc.(*auroraBackupService).makeDBSnapshots(clusterID)
		require.NoError(t, err)
		log.Infof("Waiting for snapshot %v to be ready", i+1)
		err = svc.(*auroraBackupService).checkSnapshotCreation(snapshotID)
		require.NoError(t, err)
		expectedSnapshotsIDs = append(expectedSnapshotsIDs, snapshotID)
	}

	svc.CleanUpOldBackups()
	snapshots, err := svc.(*auroraBackupService).getDBSnapshotsByPrefix()
	assert.NoError(t, err)
	assert.Len(t, snapshots, totalSnapshots)
	for _, snapshot := range snapshots {
		assert.NoError(t, err)
		assert.Contains(t, expectedSnapshotsIDs, *snapshot.DBClusterSnapshotIdentifier)
	}

	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func TestCheckSnapshotCreationNotFoundError(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	region := getAWSAccessConfig(t)
	rdsSvc, err := newRDSService(region)
	require.NoError(t, err)

	svc := auroraBackupService{
		RDS:                 rdsSvc,
		statusCheckInterval: testStatusCheckInterval,
		statusCheckAttempts: testStatusCheckAttempts,
	}

	svc.checkSnapshotCreation("a-snapshot-that-does-not-exist")
}

func TestCheckSnapshotDeletionUnexpectedStatusError(t *testing.T) {
	if !runBackupTests() {
		t.Skip("Skipping AWS RDS integration test")
	}

	region := getAWSAccessConfig(t)
	rdsSvc, err := newRDSService(region)
	require.NoError(t, err)

	svc := auroraBackupService{
		RDS:                 rdsSvc,
		snapshotIDPrefix:    testSnapshotIDPrefix,
		clusterIDPrefix:     testClusterIDPrefix,
		statusCheckInterval: testStatusCheckInterval,
		statusCheckAttempts: testStatusCheckAttempts,
	}

	clusterId, err := svc.getDBClusterID()
	require.NoError(t, err)

	snapshotID, err := svc.makeDBSnapshots(clusterId)
	require.NoError(t, err)

	err = svc.checkSnapshotCreation(snapshotID)
	require.NoError(t, err)

	err = svc.checkSnapshotDeletion(snapshotID)
	assert.EqualError(t, err, "unexpected snapshot status available")

	cleanUpTestSnapshots(t, testSnapshotIDPrefix)
}

func assertCorrectBackup(t *testing.T, snapshotIDPrefix string, expectedBackupTime time.Time) {
	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix, snapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	snapshots, err := svc.(*auroraBackupService).getDBSnapshotsByPrefix()
	assert.NoError(t, err)
	assert.Len(t, snapshots, 1)

	backupTimeLabel, err := time.Parse(snapshotIDPrefix+"-"+snapshotIDDateFormat, *snapshots[0].DBClusterSnapshotIdentifier)
	assert.NoError(t, err)
	assert.WithinDuration(t, expectedBackupTime, backupTimeLabel, 3*time.Second)
}

func cleanUpTestSnapshots(t *testing.T, snapshotIDPrefix string) {
	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix, snapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	snapshots, err := svc.(*auroraBackupService).getDBSnapshotsByPrefix()
	require.NoError(t, err)

	for _, snapshot := range snapshots {
		log.WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
			Info("cleaning up test snapshot")
		input := new(rds.DeleteDBClusterSnapshotInput)
		input.SetDBClusterSnapshotIdentifier(*snapshot.DBClusterSnapshotIdentifier)
		_, err = svc.(*auroraBackupService).RDS.DeleteDBClusterSnapshot(input)
		require.NoError(t, err)
		err = svc.(*auroraBackupService).checkSnapshotDeletion(*snapshot.DBClusterSnapshotIdentifier)
		require.NoError(t, err)
	}
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
	region := getAWSAccessConfig(t)

	svc, err := NewBackupService(region, testClusterIDPrefix, snapshotIDPrefix, testStatusCheckInterval, testStatusCheckAttempts, 0)
	require.NoError(t, err)

	snapshots, err := svc.(*auroraBackupService).getDBSnapshotsByPrefix()
	require.NoError(t, err)

	assert.Empty(t, snapshots)
}

func getAWSAccessConfig(t *testing.T) string {
	var (
		region          = os.Getenv("AWS_REGION")
		accessKeyID     = os.Getenv("AWS_ACCESS_KEY_ID")
		secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	)

	require.NotEmpty(t, region, "You need to set AWS_REGION environment variable if want to run this test")
	require.NotEmpty(t, accessKeyID, "You need to set AWS_ACCESS_KEY_ID environment variable if want to run this test")
	require.NotEmpty(t, secretAccessKey, "You need to set AWS_SECRET_ACCESS_KEY environment variable if want to run this test")

	return region
}

func runBackupTests() bool {
	runBackup := os.Getenv("RUN_BACKUP_TESTS")
	return runBackup != ""
}
