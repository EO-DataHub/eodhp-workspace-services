package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/services"
	_ "github.com/lib/pq"
)

// Workspace struct to represent the data to be inserted
// type Workspace struct {
// 	Name               string
// 	Namespace          string
// 	ServiceAccountName string
// 	AWSRoleName        string
// }

// type AWSEFSAccessPoint struct {
// 	Name        string
// 	FSID        string
// 	RootDir     string
// 	UID         int
// 	GID         int
// 	Permissions string
// }

// type AWSS3Bucket struct {
// 	BucketName      string
// 	BucketPath      string
// 	AccessPointName string
// 	EnvVar          string
// }

// type PersistentVolume struct {
// 	PVName          string
// 	StorageClass    string
// 	Size            string
// 	Driver          string
// 	AccessPointName string
// }

// type PersistentVolumeClaim struct {
// 	PVCName      string
// 	StorageClass string
// 	Size         string
// 	PVName       string
// }

// // Function to insert a workspace into the database
// func insertWorkspaceWithRelatedData(db *sql.DB, ws Workspace, efsAccessPoint AWSEFSAccessPoint, s3Bucket AWSS3Bucket, pv PersistentVolume, pvc PersistentVolumeClaim) {
// 	// Begin transaction
// 	tx, err := db.Begin()
// 	if err != nil {
// 		log.Fatalf("Error starting transaction: %v", err)
// 	}

// 	// Rollback the transaction if there's an error, will commit manually on success
// 	defer func() {
// 		if err != nil {
// 			tx.Rollback()
// 			log.Fatalf("Transaction rolled back due to: %v", err)
// 		}
// 	}()

// 	// 1. Insert into workspaces
// 	var workspaceID int
// 	sqlWorkspace := `
//     INSERT INTO dev.workspaces (ws_name, ws_namespace, ws_service_account_name, ws_aws_role_name)
//     VALUES ($1, $2, $3, $4) RETURNING id`
// 	err = tx.QueryRow(sqlWorkspace, ws.Name, ws.Namespace, ws.ServiceAccountName, ws.AWSRoleName).Scan(&workspaceID)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Printf("Inserted workspace with ID: %d\n", workspaceID)

// 	// 2. Insert into aws_efs
// 	var efsID int
// 	sqlEFS := `
// 	INSERT INTO dev.efs_access_points (workspace_id, efs_ap_name, efs_ap_fsid, efs_ap_root_directory, efs_ap_uid, efs_ap_gid, efs_ap_permissions)
// 	VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
// 	err = tx.QueryRow(sqlEFS, workspaceID, efsAccessPoint.Name, efsAccessPoint.FSID, efsAccessPoint.RootDir, efsAccessPoint.UID, efsAccessPoint.GID, efsAccessPoint.Permissions).Scan(&efsID)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Printf("Inserted AWS EFS Access Point with ID: %d\n", efsID)

// 	// 3. Insert into aws_s3_buckets
// 	sqlS3 := `
// 	INSERT INTO dev.s3_buckets (workspace_id, s3_bucket_name, s3_bucket_path, s3_ap_name, s3_env_var)
// 	VALUES ($1, $2, $3, $4, $5)`
// 	_, err = tx.Exec(sqlS3, workspaceID, s3Bucket.BucketName, s3Bucket.BucketPath, s3Bucket.AccessPointName, s3Bucket.EnvVar)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Println("Inserted AWS S3 Bucket")

// 	// 4. Insert into persistent_volumes
// 	sqlPV := `
// 	INSERT INTO dev.persistent_volumes (workspace_id, pv_name, pv_sc, pv_size, pv_driver, pv_ap_name)
// 	VALUES ($1, $2, $3, $4, $5, $6)`
// 	_, err = tx.Exec(sqlPV, workspaceID, pv.PVName, pv.StorageClass, pv.Size, pv.Driver, pv.AccessPointName)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Println("Inserted Persistent Volume")

// 	// 5. Insert into persistent_volume_claims
// 	sqlPVC := `
// 	INSERT INTO dev.persistent_volume_claims (workspace_id, pvc_name, pvc_sc, pvc_size, pv_name)
// 	VALUES ($1, $2, $3, $4, $5)`
// 	_, err = tx.Exec(sqlPVC, workspaceID, pvc.PVCName, pvc.StorageClass, pvc.Size, pvc.PVName)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Println("Inserted Persistent Volume Claim")

// 	// Commit the transaction
// 	err = tx.Commit()
// 	if err != nil {
// 		log.Fatalf("Error committing transaction: %v", err)
// 	}
// 	fmt.Println("Transaction committed successfully")
// }

func CreateWorkspace() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// Call service logic to create workspace
		services.CreateWorkspaceService(w, r)
	}
}

// func CreateWorkspace() http.HandlerFunc {

// 	return func(w http.ResponseWriter, r *http.Request) {
// 		logger := zerolog.Ctx(r.Context()).With().
// 			Str("handler", "CreateWorkspace").Logger()

// 		// Define the connection string
// 		connStr := "postgresql://workspaces-dev-ILzXv3:fXYCtshu8G5oFCy@localhost:8443/workspaces"

// 		// Connect to the PostgreSQL database
// 		db, err := sql.Open("postgres", connStr)
// 		if err != nil {
// 			log.Fatalf("Error connecting to the database: %v", err)
// 		}
// 		defer db.Close()

// 		workspace := Workspace{
// 			Name:               "dev-user",
// 			Namespace:          "ws-dev-user",
// 			ServiceAccountName: "default",
// 			AWSRoleName:        "eodhp-dev-y4jFxoD4-dev-user",
// 		}

// 		efsAccessPoint := AWSEFSAccessPoint{
// 			Name:        "eodhp-dev-y4jFxoD4-dev-user-pv",
// 			FSID:        "fs-045e65dcd4e24f91d",
// 			RootDir:     "/workspaces/dev-user",
// 			UID:         1000,
// 			GID:         1000,
// 			Permissions: "0755",
// 		}

// 		s3Bucket := AWSS3Bucket{
// 			BucketName:      "eodhp-dev-workspaces",
// 			BucketPath:      "dev-user/",
// 			AccessPointName: "eodhp-dev-y4jFxoD4-dev-user-s3",
// 			EnvVar:          "S3_BUCKET_WORKSPACE",
// 		}

// 		persistentVolume := PersistentVolume{
// 			PVName:          "pv-dev-user-workspace",
// 			StorageClass:    "file-storage",
// 			Size:            "10Gi",
// 			Driver:          "efs.csi.aws.com",
// 			AccessPointName: "eodhp-dev-y4jFxoD4-dev-user-pv",
// 		}

// 		persistentVolumeClaim := PersistentVolumeClaim{
// 			PVCName:      "pvc-workspace",
// 			StorageClass: "file-storage",
// 			Size:         "10Gi",
// 			PVName:       "pv-dev-user-workspace",
// 		}

// 		insertWorkspaceWithRelatedData(db, workspace, efsAccessPoint, s3Bucket, persistentVolume, persistentVolumeClaim)

// 		logger.Info().Msg("Workspace created...")

// 	}

// }
