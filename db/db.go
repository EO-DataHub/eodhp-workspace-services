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
			name VARCHAR(255) UNIQUE NOT NULL,
			account UUID NOT NULL,
			accountOwner VARCHAR(255) NOT NULL,
			memberGroup VARCHAR(255) NOT NULL,
			roleName VARCHAR(255) NOT NULL,
			roleArn VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table workspaces")

		tx.Rollback()
		return err
	}

	// Superclass table for workspace stores (both object stores and block stores)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS workspace_stores (
			id UUID PRIMARY KEY,
			workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
			store_type VARCHAR(50) NOT NULL, 		-- object or block
			name VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table workspace_stores")

		tx.Rollback()
		return err
	}

	// Subclass table for object stores (inherits from workspace_stores)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS object_stores (
			store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
			path VARCHAR(255) NOT NULL,
			envVar VARCHAR(255) NOT NULL,
			accessPointArn VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table object_stores")

		tx.Rollback()
		return err
	}

	// Subclass table for block stores (inherits from workspace_stores)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS block_stores (
			store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
			accessPointId VARCHAR(255) NOT NULL,  
			fsId VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table block_stores")

		tx.Rollback()
		return err
	}

	// Commit the transaction to persist changes
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	w.Log.Info().Msg("Tables initialized successfully")
	return nil
}

func (w *WorkspaceDB) InsertWorkspace(ack *models.AckPayload) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback() // rollback if anything fails

	// Insert the workspace
	workspaceID := uuid.New() // Generate a new workspace ID
	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, name, account, accountOwner, memberGroup, roleName, roleArn)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		workspaceID, ack.MessagePayload.Name, ack.MessagePayload.Account, ack.MessagePayload.AccountOwner,
		ack.MessagePayload.MemberGroup, ack.AWS.Role.Name, ack.AWS.Role.ARN)
	if err != nil {
		w.Log.Error().Err(err).Msg("error inserting workspace")
		return fmt.Errorf("error inserting workspace: %v", err)
	}

	// Insert object stores
	for _, bucket := range ack.AWS.S3.Buckets {
		storeID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO workspace_stores (id, workspace_id, store_type, name)
			VALUES ($1, $2, 'object', $3)`,
			storeID, workspaceID, bucket.Name)
		if err != nil {
			w.Log.Error().Err(err).Msg("error inserting into workspace_stores")
			return fmt.Errorf("error inserting into workspace_stores: %v", err)
		}

		err = w.execQuery(tx, `
			INSERT INTO object_stores (store_id, path, envVar, accessPointArn)
			VALUES ($1, $2, $3, $4)`,
			storeID, bucket.Path, bucket.EnvVar, bucket.AccessPointARN)
		if err != nil {
			w.Log.Error().Err(err).Msg("error inserting into object_stores")
			return fmt.Errorf("error inserting into object_stores: %v", err)
		}
	}

	// Insert block stores
	for _, accessPoint := range ack.AWS.EFS.AccessPoints {
		storeID := uuid.New()
		err = w.execQuery(tx, `
			INSERT INTO workspace_stores (id, workspace_id, store_type, name)
			VALUES ($1, $2, 'block', $3)`,
			storeID, workspaceID, accessPoint.Name)
		if err != nil {
			w.Log.Error().Err(err).Msg("error inserting into workspace_stores")
			return fmt.Errorf("error inserting into workspace_stores: %v", err)
		}

		err = w.execQuery(tx, `
			INSERT INTO block_stores (store_id, accessPointId, fsId)
			VALUES ($1, $2, $3)`,
			storeID, accessPoint.AccessPointID, accessPoint.FSID)
		if err != nil {
			w.Log.Error().Err(err).Msg("error inserting into block_stores")
			return fmt.Errorf("error inserting into block_stores: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	w.Log.Info().Msg("Workspace and stores inserted successfully")
	return nil
}

func (w *WorkspaceDB) DeleteWorkspace(workspaceID uuid.UUID) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Delete the workspace (this will also delete the associated stores due to ON DELETE CASCADE)
	err = w.execQuery(tx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	if err != nil {
		w.Log.Error().Err(err).Msg("error deleting workspace")
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	w.Log.Info().Msg("Workspace deleted successfully")
	return nil
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
