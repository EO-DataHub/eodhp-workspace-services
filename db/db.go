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
			accountOwner TEXT NOT NULL,
			memberGroup TEXT NOT NULL,
			roleName TEXT NOT NULL,
			roleArn TEXT NOT NULL,
			status TEXT NOT NULL
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

func (w *WorkspaceDB) InsertWorkspace(req *models.ReqMessagePayload) (*sql.Tx, error) {
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}

	// Insert the workspace
	workspaceID := uuid.New() // Generate a new workspace ID
	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, name, account, accountOwner, memberGroup, status)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		workspaceID, req.Name, req.Account, req.AccountOwner,
		req.MemberGroup, req.Status)
	if err != nil {
		w.Log.Error().Err(err).Msg("error inserting workspace")
		return nil, fmt.Errorf("error inserting workspace: %v", err)
	}

	return tx, nil
}

func (w *WorkspaceDB) CommitTransaction(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}
	w.Log.Info().Msg("Transaction committed successfully")
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
