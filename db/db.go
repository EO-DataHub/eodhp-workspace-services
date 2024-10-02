package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type WorkspaceDB struct {
	DB     *sql.DB
	Events events.Notifier
	Log    *zerolog.Logger
}

// NewWorkspaceDB is a constructor that initializes WorkspaceDB with DB and Log
func NewWorkspaceDB(events events.Notifier, log *zerolog.Logger) (*WorkspaceDB, error) {
	// Get the database connection string from the environment
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Error().Msg("DATABASE_URL environment variable is not set")
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	// Open the database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open database connection")
		return nil, err
	}

	// Check we are actually connected
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Database connection failed during ping")
		return nil, err
	}

	return &WorkspaceDB{
		DB:     db,
		Events: events,
		Log:    log,
	}, nil
}

func (w *WorkspaceDB) Close() error {
	if err := w.DB.Close(); err != nil {
		return err
	}
	w.Log.Info().Msg("database connection closed")

	w.Events.Close()
	w.Log.Info().Msg("event publisher closed")
	w.DB = nil
	w.Events = nil
	w.Log = nil

	return nil
}

func (w *WorkspaceDB) InitTables() error {

	err := w.DB.Ping()
	if err != nil {
		w.Log.Error().Err(err).Msg("Database connection ping failed")
		return fmt.Errorf("database connection ping failed: %v", err)
	}

	w.Log.Debug().Msg("Database connection is healthy, starting table initialization")

	tx, err := w.DB.Begin()
	if err != nil {
		w.Log.Error().Err(err).Msg("error starting transaction")
		return fmt.Errorf("error starting transaction: %v", err)
	}

	// Create the workspaces table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS workspaces (
			id UUID PRIMARY KEY,
			ws_name VARCHAR(255) UNIQUE NOT NULL,     
			ws_namespace VARCHAR(255) NOT NULL,         
			ws_service_account_name VARCHAR(255) NOT NULL,
			ws_aws_role_name VARCHAR(255)
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating efs_access_points table")

		tx.Rollback()
		return fmt.Errorf("error creating workspaces table: %v", err)
	}

	// Create the efs_access_points table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS efs_access_points (
			id UUID PRIMARY KEY,
			workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
			efs_ap_name VARCHAR(255) NOT NULL,
			efs_ap_fsid VARCHAR(255) NOT NULL,
			efs_ap_root_directory VARCHAR(255),
			efs_ap_uid INT,
			efs_ap_gid INT,
			efs_ap_permissions VARCHAR(10)
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating efs_access_points table")

		tx.Rollback()
		return fmt.Errorf("error creating efs_access_points table: %v", err)
	}

	// Create the s3_buckets table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS s3_buckets (
			id UUID PRIMARY KEY,
			workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
			s3_bucket_name VARCHAR(255) NOT NULL,  
			s3_bucket_path VARCHAR(255),        
			s3_ap_name VARCHAR(255), 
			s3_env_var VARCHAR(255)
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating efs_access_points table")

		tx.Rollback()
		return fmt.Errorf("error creating s3_buckets table: %v", err)
	}

	// Create the persistent_volumes table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS persistent_volumes (
			id UUID PRIMARY KEY,
			workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE, 
			pv_name VARCHAR(255) NOT NULL,  
			pv_sc VARCHAR(255),            
			pv_size VARCHAR(10) NOT NULL,      
			pv_driver VARCHAR(255),           
			pv_ap_name VARCHAR(255)
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating efs_access_points table")

		tx.Rollback()
		return fmt.Errorf("error creating persistent_volumes table: %v", err)
	}

	// Create the persistent_volume_claims table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS persistent_volume_claims (
			id UUID PRIMARY KEY,
			workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE, 
			pvc_name VARCHAR(255) NOT NULL,       
			pvc_sc VARCHAR(255),            
			pvc_size VARCHAR(10) NOT NULL,        
			pv_name VARCHAR(255)
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating efs_access_points table")

		tx.Rollback()
		return fmt.Errorf("error creating persistent_volume_claims table: %v", err)
	}

	// Commit the transaction to persist changes
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	w.Log.Info().Msg("Tables initialized successfully")
	return nil
}

// inserts a workspace and its related data into the database - will rollback if not all statements executed
func (w *WorkspaceDB) InsertWorkspace(ws models.Workspace,
	efsAccessPoints []models.AWSEFSAccessPoint,
	s3Buckets []models.AWSS3Bucket,
	pvs []models.PersistentVolume,
	pvcs []models.PersistentVolumeClaim) (uuid.UUID, error) {

	// Generate UUID for the workspace
	workspaceID := uuid.New()

	tx, err := w.DB.Begin()
	if err != nil {
		return uuid.Nil, fmt.Errorf("error starting transaction: %v", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert workspace and get the workspace ID
	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, ws_name, ws_namespace, ws_service_account_name, ws_aws_role_name)
		VALUES ($1, $2, $3, $4, $5)`,
		workspaceID, ws.Name, ws.Namespace, ws.ServiceAccountName, ws.AWSRoleName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error inserting workspace: %v", err)
	}
	fmt.Printf("Inserted workspace with ID: %s\n", workspaceID)

	// Insert multiple AWS EFS Access Points
	for _, efs := range efsAccessPoints {
		efsID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO efs_access_points (id, workspace_id, efs_ap_name, efs_ap_fsid, efs_ap_root_directory, efs_ap_uid, efs_ap_gid, efs_ap_permissions)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			efsID, workspaceID, efs.Name, efs.FSID, efs.RootDir, efs.UID, efs.GID, efs.Permissions)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting AWS EFS Access Point: %v", err)
		}
	}

	// Insert multiple AWS S3 Buckets
	for _, bucket := range s3Buckets {
		s3BucketID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO s3_buckets (id, workspace_id, s3_bucket_name, s3_bucket_path, s3_ap_name, s3_env_var)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			s3BucketID, workspaceID, bucket.BucketName, bucket.BucketPath, bucket.AccessPointName, bucket.EnvVar)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting AWS S3 Bucket: %v", err)
		}
	}

	// Insert multiple Persistent Volumes
	for _, pv := range pvs {
		pvID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO persistent_volumes (id, workspace_id, pv_name, pv_sc, pv_size, pv_driver, pv_ap_name)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			pvID, workspaceID, pv.PVName, pv.StorageClass, pv.Size, pv.Driver, pv.AccessPointName)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting Persistent Volume: %v", err)
		}
	}

	// Insert multiple Persistent Volume Claims
	for _, pvc := range pvcs {
		pvcID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO persistent_volume_claims (id, workspace_id, pvc_name, pvc_sc, pvc_size, pv_name)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			pvcID, workspaceID, pvc.PVCName, pvc.StorageClass, pvc.Size, pvc.PVName)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting Persistent Volume Claim: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("error committing transaction: %v", err)
	}
	w.Log.Info().Msg("Transaction committed successfully for workspaceID: " + workspaceID.String())
	return workspaceID, nil
}

func (w *WorkspaceDB) execQuery(tx *sql.Tx, query string, args ...interface{}) error {

	if w.DB == nil {
		return fmt.Errorf("database connection is not established")
	}

	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	return nil
}
