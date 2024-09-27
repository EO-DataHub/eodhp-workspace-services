package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	_ "github.com/lib/pq"
)

// Establishes a connection to the workspaces database
func ConnectPostgres() (*sql.DB, error) {

	//connStr := os.Getenv("DATABASE_URL")
	connStr := "postgresql://workspaces-dev-ILzXv3:fXYCtshu8G5oFCy@localhost:8443/workspaces"
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening connection to database: %w", err)
	}

	// Ping the database to ensure the connection is established
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging the database: %w", err)
	}

	log.Println("Successfully connected to the PostgreSQL database")

	return db, nil
}

// inserts a workspace and its related data into the database - will rollback if not all statements executed
func InsertWorkspaceWithRelatedData(db *sql.DB, ws models.Workspace, efsAccessPoints []models.AWSEFSAccessPoint, s3Buckets []models.AWSS3Bucket, pvs []models.PersistentVolume, pvcs []models.PersistentVolumeClaim) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert workspace and get the workspace ID
	workspaceID, err := insertAndReturnID(tx, `
		INSERT INTO dev.workspaces (ws_name, ws_namespace, ws_service_account_name, ws_aws_role_name)
		VALUES ($1, $2, $3, $4) RETURNING id`, ws.Name, ws.Namespace, ws.ServiceAccountName, ws.AWSRoleName)
	if err != nil {
		return err
	}
	fmt.Printf("Inserted workspace with ID: %d\n", workspaceID)

	// Insert multiple AWS EFS Access Points
	for _, efs := range efsAccessPoints {
		_, err = insertAndReturnID(tx, `
			INSERT INTO dev.efs_access_points (workspace_id, efs_ap_name, efs_ap_fsid, efs_ap_root_directory, efs_ap_uid, efs_ap_gid, efs_ap_permissions)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			workspaceID, efs.Name, efs.FSID, efs.RootDir, efs.UID, efs.GID, efs.Permissions)
		if err != nil {
			return err
		}
		fmt.Printf("Inserted AWS EFS Access Point: %s\n", efs.Name)
	}

	// Insert multiple AWS S3 Buckets
	for _, bucket := range s3Buckets {
		if err := execQuery(tx, `
			INSERT INTO dev.s3_buckets (workspace_id, s3_bucket_name, s3_bucket_path, s3_ap_name, s3_env_var)
			VALUES ($1, $2, $3, $4, $5)`,
			workspaceID, bucket.BucketName, bucket.BucketPath, bucket.AccessPointName, bucket.EnvVar); err != nil {
			return err
		}
		fmt.Printf("Inserted AWS S3 Bucket: %s\n", bucket.BucketName)
	}

	// Insert multiple Persistent Volumes
	for _, pv := range pvs {
		if err := execQuery(tx, `
			INSERT INTO dev.persistent_volumes (workspace_id, pv_name, pv_sc, pv_size, pv_driver, pv_ap_name)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			workspaceID, pv.PVName, pv.StorageClass, pv.Size, pv.Driver, pv.AccessPointName); err != nil {
			return err
		}
		fmt.Printf("Inserted Persistent Volume: %s\n", pv.PVName)
	}

	// Insert multiple Persistent Volume Claims
	for _, pvc := range pvcs {
		if err := execQuery(tx, `
			INSERT INTO dev.persistent_volume_claims (workspace_id, pvc_name, pvc_sc, pvc_size, pv_name)
			VALUES ($1, $2, $3, $4, $5)`,
			workspaceID, pvc.PVCName, pvc.StorageClass, pvc.Size, pvc.PVName); err != nil {
			return err
		}
		fmt.Printf("Inserted Persistent Volume Claim: %s\n", pvc.PVCName)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}
	fmt.Println("Transaction committed successfully")
	return nil
}

// Inserts data and returns the ID for a given query
func insertAndReturnID(tx *sql.Tx, query string, args ...interface{}) (int, error) {
	var id int
	err := tx.QueryRow(query, args...).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert data: %v", err)
	}
	return id, nil
}

// execQuery executes a query
func execQuery(tx *sql.Tx, query string, args ...interface{}) error {
	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	return nil
}
