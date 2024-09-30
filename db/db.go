package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	_ "github.com/lib/pq"
)

// Establishes a connection to the workspaces database
func ConnectPostgres() (*sql.DB, error) {

	connStr := os.Getenv("DATABASE_URL")

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

func InitTables() {
	dbConn, err := ConnectPostgres()
	if err != nil {
		log.Println("Database connection error:", err)
		return
	}
	defer dbConn.Close()

	tx, err := dbConn.Begin()

	if err != nil {
		fmt.Printf("error starting transaction: %s", err)
	}

	execQuery(tx,
		`
		CREATE TABLE IF NOT EXISTS workspaces 
		(
			id SERIAL PRIMARY KEY,
			ws_name VARCHAR(255) UNIQUE NOT NULL,     
			ws_namespace VARCHAR(255) NOT NULL,         
			ws_service_account_name VARCHAR(255) NOT NULL,
			ws_aws_role_name VARCHAR(255)
		);
		CREATE TABLE IF NOT EXISTS efs_access_points 
		(
    		id SERIAL PRIMARY KEY,
    		workspace_id INT REFERENCES dev.workspaces(id) ON DELETE CASCADE,
    		efs_ap_name VARCHAR(255) NOT NULL,
    		efs_ap_fsid VARCHAR(255) NOT NULL,
    		efs_ap_root_directory VARCHAR(255),
    		efs_ap_uid INT,
    		efs_ap_gid INT,
    		efs_ap_permissions VARCHAR(10)
		);
		CREATE TABLE IF NOT EXISTS s3_buckets 
		(
    		id SERIAL PRIMARY KEY,
    		workspace_id INTEGER REFERENCES dev.workspaces(id) ON DELETE CASCADE,
    		s3_bucket_name VARCHAR(255) NOT NULL,  
    		s3_bucket_path VARCHAR(255),        
    		s3_ap_name VARCHAR(255), 
    		s3_env_var VARCHAR(255)     
		);
		CREATE TABLE IF NOT EXISTS persistent_volumes 
		(
			id SERIAL PRIMARY KEY,
			workspace_id INTEGER REFERENCES dev.workspaces(id) ON DELETE CASCADE, 
			pv_name VARCHAR(255) NOT NULL,  
			pv_sc VARCHAR(255),            
			pv_size VARCHAR(10) NOT NULL,      
			pv_driver VARCHAR(255),           
			pv_ap_name VARCHAR(255)      
		);
		CREATE TABLE IF NOT EXISTS persistent_volume_claims 
		(
			id SERIAL PRIMARY KEY,
			workspace_id INTEGER REFERENCES dev.workspaces(id) ON DELETE CASCADE, 
			pvc_name VARCHAR(255) NOT NULL,       
			pvc_sc VARCHAR(255),            
			pvc_size VARCHAR(10) NOT NULL,        
			pv_name VARCHAR(255)     
		);
	`)

	// Commit the transaction to persist changes
	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return
	}
}

// inserts a workspace and its related data into the database - will rollback if not all statements executed
func InsertWorkspaceWithRelatedData(db *sql.DB, ws models.Workspace,
	efsAccessPoints []models.AWSEFSAccessPoint,
	s3Buckets []models.AWSS3Bucket,
	pvs []models.PersistentVolume,
	pvcs []models.PersistentVolumeClaim) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
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
		return 0, err
	}
	fmt.Printf("Inserted workspace with ID: %d\n", workspaceID)

	// Insert multiple AWS EFS Access Points
	for _, efs := range efsAccessPoints {
		_, err = insertAndReturnID(tx, `
			INSERT INTO dev.efs_access_points (workspace_id, efs_ap_name, efs_ap_fsid, efs_ap_root_directory, efs_ap_uid, efs_ap_gid, efs_ap_permissions)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			workspaceID, efs.Name, efs.FSID, efs.RootDir, efs.UID, efs.GID, efs.Permissions)
		if err != nil {
			return 0, err
		}
		fmt.Printf("Inserted AWS EFS Access Point: %s\n", efs.Name)
	}

	// Insert multiple AWS S3 Buckets
	for _, bucket := range s3Buckets {
		if err := execQuery(tx, `
			INSERT INTO dev.s3_buckets (workspace_id, s3_bucket_name, s3_bucket_path, s3_ap_name, s3_env_var)
			VALUES ($1, $2, $3, $4, $5)`,
			workspaceID, bucket.BucketName, bucket.BucketPath, bucket.AccessPointName, bucket.EnvVar); err != nil {
			return 0, err
		}
		fmt.Printf("Inserted AWS S3 Bucket: %s\n", bucket.BucketName)
	}

	// Insert multiple Persistent Volumes
	for _, pv := range pvs {
		if err := execQuery(tx, `
			INSERT INTO dev.persistent_volumes (workspace_id, pv_name, pv_sc, pv_size, pv_driver, pv_ap_name)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			workspaceID, pv.PVName, pv.StorageClass, pv.Size, pv.Driver, pv.AccessPointName); err != nil {
			return 0, err
		}
		fmt.Printf("Inserted Persistent Volume: %s\n", pv.PVName)
	}

	// Insert multiple Persistent Volume Claims
	for _, pvc := range pvcs {
		if err := execQuery(tx, `
			INSERT INTO dev.persistent_volume_claims (workspace_id, pvc_name, pvc_sc, pvc_size, pv_name)
			VALUES ($1, $2, $3, $4, $5)`,
			workspaceID, pvc.PVCName, pvc.StorageClass, pvc.Size, pvc.PVName); err != nil {
			return 0, err
		}
		fmt.Printf("Inserted Persistent Volume Claim: %s\n", pvc.PVCName)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}
	fmt.Println("Transaction committed successfully")
	return workspaceID, nil
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
