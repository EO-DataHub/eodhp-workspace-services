package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

// Workspace struct to represent the data to be inserted
type Workspace struct {
	Name               string
	Namespace          string
	ServiceAccountName string
	AWSRoleName        *string
}

type AWSEFS struct {
	FSID        string
	RootDir     string
	UID         int
	GID         int
	Permissions string
}

type AWSEFSAccessPoint struct {
	Name string
}

type AWSS3Bucket struct {
	BucketName      string
	BucketPath      *string
	AccessPointName *string
	EnvVar          *string
}

type PersistentVolume struct {
	PVName          string
	StorageClass    *string
	Size            string
	Driver          *string
	AccessPointName *string
}

type PersistentVolumeClaim struct {
	PVCName      string
	StorageClass *string
	Size         string
	PVName       *string
}

// Function to insert a workspace into the database
func insertWorkspaceWithRelatedData(db *sql.DB, ws Workspace, efs AWSEFS, efsAccessPoint AWSEFSAccessPoint, s3Bucket AWSS3Bucket, pv PersistentVolume, pvc PersistentVolumeClaim) {
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}

	// Rollback the transaction if there's an error, will commit manually on success
	defer func() {
		if err != nil {
			tx.Rollback()
			log.Fatalf("Transaction rolled back due to: %v", err)
		}
	}()

	// 1. Insert into workspaces
	var workspaceID int
	sqlWorkspace := `
    INSERT INTO dev.workspaces (name, namespace, service_account_name, aws_role_name)
    VALUES ($1, $2, $3, $4) RETURNING id`
	err = tx.QueryRow(sqlWorkspace, ws.Name, ws.Namespace, ws.ServiceAccountName, ws.AWSRoleName).Scan(&workspaceID)
	if err != nil {
		return
	}
	fmt.Printf("Inserted workspace with ID: %d\n", workspaceID)

	// 2. Insert into aws_efs
	var efsID int
	sqlEFS := `
    INSERT INTO dev.aws_efs (workspace_id, fs_id, root_directory, uid, gid, permissions)
    VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err = tx.QueryRow(sqlEFS, workspaceID, efs.FSID, efs.RootDir, efs.UID, efs.GID, efs.Permissions).Scan(&efsID)
	if err != nil {
		return
	}
	fmt.Printf("Inserted AWS EFS with ID: %d\n", efsID)

	// 3. Insert into aws_efs_access_points
	sqlEFSAccessPoint := `
    INSERT INTO dev.aws_efs_access_points (efs_id, name)
    VALUES ($1, $2)`
	_, err = tx.Exec(sqlEFSAccessPoint, efsID, efsAccessPoint.Name)
	if err != nil {
		return
	}
	fmt.Println("Inserted AWS EFS Access Point")

	// 4. Insert into aws_s3_buckets
	sqlS3 := `
    INSERT INTO dev.aws_s3_buckets (workspace_id, bucket_name, bucket_path, access_point_name, env_var)
    VALUES ($1, $2, $3, $4, $5)`
	_, err = tx.Exec(sqlS3, workspaceID, s3Bucket.BucketName, s3Bucket.BucketPath, s3Bucket.AccessPointName, s3Bucket.EnvVar)
	if err != nil {
		return
	}
	fmt.Println("Inserted AWS S3 Bucket")

	// 5. Insert into persistent_volumes
	sqlPV := `
    INSERT INTO dev.persistent_volumes (workspace_id, pv_name, storage_class, size, driver, access_point_name)
    VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.Exec(sqlPV, workspaceID, pv.PVName, pv.StorageClass, pv.Size, pv.Driver, pv.AccessPointName)
	if err != nil {
		return
	}
	fmt.Println("Inserted Persistent Volume")

	// 6. Insert into persistent_volume_claims
	sqlPVC := `
    INSERT INTO dev.persistent_volume_claims (workspace_id, pvc_name, storage_class, size, pv_name)
    VALUES ($1, $2, $3, $4, $5)`
	_, err = tx.Exec(sqlPVC, workspaceID, pvc.PVCName, pvc.StorageClass, pvc.Size, pvc.PVName)
	if err != nil {
		return
	}
	fmt.Println("Inserted Persistent Volume Claim")

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}
	fmt.Println("Transaction committed successfully")
}

func CreateWorkspace() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context()).With().
			Str("handler", "CreateWorkspace").Logger()

		// Define the connection string
		connStr := "postgresql://workspaces-dev-ILzXv3:fXYCtshu8G5oFCy@localhost:8443/workspaces"

		// Connect to the PostgreSQL database
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			log.Fatalf("Error connecting to the database: %v", err)
		}
		defer db.Close()

		workspace := Workspace{
			Name:               "example-workspace2",
			Namespace:          "default",
			ServiceAccountName: "example-service-account",
			AWSRoleName:        nil,
		}

		awsEFS := AWSEFS{
			FSID:        "fs-example",
			RootDir:     "/root/dir",
			UID:         1000,
			GID:         1000,
			Permissions: "0755",
		}

		efsAccessPoint := AWSEFSAccessPoint{
			Name: "efs-access-point",
		}

		s3Bucket := AWSS3Bucket{
			BucketName:      "example-bucket",
			BucketPath:      nil,
			AccessPointName: nil,
			EnvVar:          nil,
		}

		persistentVolume := PersistentVolume{
			PVName:          "example-pv",
			StorageClass:    nil,
			Size:            "10Gi",
			Driver:          nil,
			AccessPointName: nil,
		}

		persistentVolumeClaim := PersistentVolumeClaim{
			PVCName:      "example-pvc",
			StorageClass: nil,
			Size:         "10Gi",
			PVName:       nil,
		}

		insertWorkspaceWithRelatedData(db, workspace, awsEFS, efsAccessPoint, s3Bucket, persistentVolume, persistentVolumeClaim)

		logger.Info().Msg("Workspace created...")

	}

}
