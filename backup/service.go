package backup

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
)

const snapshotIDDateFormat = "2006-01-02-15-04-05"

const statusAvailable = "available"
const statusCreating = "creating"
const statusDeleting = "deleting"
const statusDeleted = "deleted"

type Service interface {
	MakeBackup()
	CleanUpOldBackups()
}

type auroraBackupService struct {
	*rds.RDS
	clusterIDPrefix     string
	snapshotIDPrefix    string
	statusCheckInterval time.Duration
	statusCheckAttempts int
	backupsRetention    int
}

func NewBackupService(region, accessKeyID, secretAccessKey, clusterIDPrefix, snapshotIDPrefix string, statusCheckInterval time.Duration, statusCheckAttempts, backupsRetention int) (Service, error) {
	svc, err := newRDSService(region, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}
	return &auroraBackupService{
		svc,
		clusterIDPrefix,
		snapshotIDPrefix,
		statusCheckInterval,
		statusCheckAttempts,
		backupsRetention,
	}, nil
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

func (svc *auroraBackupService) MakeBackup() {
	log.Info("Getting DB cluster ID")
	clusterID, err := svc.getDBClusterID()
	if err != nil {
		log.WithError(err).Error("Error in fetching DB cluster information from AWS")
		return
	}

	log.WithField("clusterID", clusterID).
		Info("Making snapshot for cluster")
	snapshotID, err := svc.makeDBSnapshots(clusterID)
	if err != nil {
		log.WithError(err).Error("Error in creating DB snapshot")
		return
	}

	log.WithField("snapshotID", snapshotID).
		Info("Checking for snapshot successfully created")
	err = svc.checkSnapshotCreation(snapshotID)
	if err != nil {
		log.WithField("snapshotID", snapshotID).
			WithError(err).
			Error("Error in snapshot creation check")
		return
	}

	log.WithField("snapshotID", snapshotID).Info("PAC aurora backup successfully created")
}

func (svc *auroraBackupService) getDBClusterID() (string, error) {
	clusterIdentifierPrefix := svc.clusterIDPrefix
	isLastPage := false
	input := new(rds.DescribeDBClustersInput)
	for !isLastPage {
		result, err := svc.DescribeDBClusters(input)
		if err != nil {
			return "", err
		}
		for _, cluster := range result.DBClusters {
			if strings.HasPrefix(*cluster.DBClusterIdentifier, clusterIdentifierPrefix) {
				return *cluster.DBClusterIdentifier, nil
			}
		}
		if result.Marker != nil {
			input.SetMarker(*result.Marker)
		} else {
			isLastPage = true
		}
	}
	return "", fmt.Errorf("DB cluster not found with identifier prefix %v", clusterIdentifierPrefix)
}

func (svc *auroraBackupService) makeDBSnapshots(clusterID string) (string, error) {
	input := new(rds.CreateDBClusterSnapshotInput)
	input.SetDBClusterIdentifier(clusterID)
	timestamp := time.Now().Format(snapshotIDDateFormat)
	snapshotIdentifier := svc.snapshotIDPrefix + "-" + timestamp
	input.SetDBClusterSnapshotIdentifier(snapshotIdentifier)

	_, err := svc.CreateDBClusterSnapshot(input)

	return snapshotIdentifier, err
}

func (svc *auroraBackupService) checkSnapshotCreation(snapshotID string) error {
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetDBClusterSnapshotIdentifier(snapshotID)

	for attempt := 0; attempt < svc.statusCheckAttempts; attempt++ {
		time.Sleep(svc.statusCheckInterval)
		result, err := svc.DescribeDBClusterSnapshots(input)
		if err != nil {
			return err
		}
		if len(result.DBClusterSnapshots) < 1 {
			return errors.New("snapshot not found")
		}
		if *result.DBClusterSnapshots[0].Status != statusCreating {
			if *result.DBClusterSnapshots[0].Status == statusAvailable {
				return nil
			} else {
				return fmt.Errorf("unexpected snapshot status %v", *result.DBClusterSnapshots[0].Status)
			}
		}
	}
	return errors.New("check for snapshot creation time out")
}

func (svc *auroraBackupService) CleanUpOldBackups() {
	log.Info("Getting list of snapshot to be cleaned up")
	snapshots, err := svc.getDBSnapshotsByPrefix()
	if err != nil {
		log.WithError(err).Error("Error in fetching DB cluster snapshots for cleanup")
		return
	}

	if len(snapshots) > svc.backupsRetention {
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].SnapshotCreateTime.After(*snapshots[j].SnapshotCreateTime)
		})
		snapshots = snapshots[svc.backupsRetention:]

		for _, snapshot := range snapshots {
			log.WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
				Info("Deleting snapshot for cleanup")
			input := new(rds.DeleteDBClusterSnapshotInput)
			input.SetDBClusterSnapshotIdentifier(*snapshot.DBClusterSnapshotIdentifier)
			_, err = svc.DeleteDBClusterSnapshot(input)
			if err != nil {
				log.WithError(err).
					WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
					Error("Error in deleting DB cluster snapshot for cleanup")
			} else {
				log.WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
					Info("Checking for snapshot successfully deleted")
				err = svc.checkSnapshotDeletion(*snapshot.DBClusterSnapshotIdentifier)
				if err != nil {
					log.WithError(err).
						WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
						Error("Error in checking DB cluster snapshot deletion for cleanup")
				}
				log.WithField("snapshotID", *snapshot.DBClusterSnapshotIdentifier).
					Info("Deleted old snapshot for cleanup")
			}
		}
	}
}

func (svc *auroraBackupService) getDBSnapshotsByPrefix() ([]*rds.DBClusterSnapshot, error) {
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
			if strings.HasPrefix(*snapshot.DBClusterSnapshotIdentifier, svc.snapshotIDPrefix) {
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

func (svc *auroraBackupService) checkSnapshotDeletion(snapshotID string) error {
	input := new(rds.DescribeDBClusterSnapshotsInput)
	input.SetDBClusterSnapshotIdentifier(snapshotID)

	for attempt := 0; attempt < svc.statusCheckAttempts; attempt++ {
		time.Sleep(svc.statusCheckInterval)
		result, err := svc.DescribeDBClusterSnapshots(input)
		if err != nil {
			switch err.(type) {
			case awserr.Error:
				if err.(awserr.Error).Code() == rds.ErrCodeDBClusterSnapshotNotFoundFault {
					return nil
				} else {
					return err
				}
			default:
				return err
			}
		}
		if len(result.DBClusterSnapshots) == 0 {
			return nil
		}
		if *result.DBClusterSnapshots[0].Status != statusDeleting {
			if *result.DBClusterSnapshots[0].Status == statusDeleted {
				return nil
			} else {
				return fmt.Errorf("unexpected snapshot status %v", *result.DBClusterSnapshots[0].Status)
			}
		}
	}
	return errors.New("check for snapshot deletion time out")
}
