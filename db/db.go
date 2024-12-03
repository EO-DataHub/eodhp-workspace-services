package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// WorkspaceDB wraps database, events, and logging functionalities.
type WorkspaceDB struct {
	DB *sql.DB
}

// NewWorkspaceDB initializes a WorkspaceDB instance with a database connection.
func NewWorkspaceDB() (*WorkspaceDB, error) {

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Error().Msg("DATABASE_URL environment variable is not set")
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	// Open and verify database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open database connection")
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Database connection failed during ping")
		return nil, err
	}

	return &WorkspaceDB{
		DB: db,
	}, nil
}

// Close closes the database connection and event notifier.
func (w *WorkspaceDB) Close() error {
	return w.DB.Close()
}

// InitTables creates necessary tables if they do not already exist.
func (w *WorkspaceDB) InitTables() error {

	err := w.DB.Ping()
	if err != nil {
		log.Error().Err(err).Msg("Database connection ping failed")
		return fmt.Errorf("database connection ping failed: %w", err)
	}

	log.Debug().Msg("Database connection is healthy, starting table initialization")

	tx, err := w.DB.Begin()
	if err != nil {
		log.Error().Err(err).Msg("error starting transaction")
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// List of table creation queries
	createTableQueries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
				id UUID PRIMARY KEY,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				name VARCHAR(255) NOT NULL,
				account_owner TEXT NOT NULL
			);`,
		`CREATE TABLE IF NOT EXISTS workspaces (
				id UUID PRIMARY KEY,
				name VARCHAR(255) UNIQUE NOT NULL,
				account UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				member_group TEXT NOT NULL,
				role_name TEXT NULL,
				role_arn TEXT NULL,
				status TEXT NOT NULL,
				last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
			);`,
		`CREATE TABLE IF NOT EXISTS workspace_stores (
				id UUID PRIMARY KEY,
				workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
				store_type VARCHAR(50) NOT NULL,
				name VARCHAR(255) NOT NULL
			);`,
		`CREATE TABLE IF NOT EXISTS object_stores (
				store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
				path VARCHAR(255) NOT NULL,
				env_var VARCHAR(255) NOT NULL,
				access_point_arn VARCHAR(255) NOT NULL
			);`,
		`CREATE TABLE IF NOT EXISTS block_stores (
				store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
				access_point_id VARCHAR(255) NOT NULL,
				fs_id VARCHAR(255) NOT NULL
			);`,
	}

	// Execute each table creation query in the transaction
	for _, query := range createTableQueries {
		if _, err := tx.Exec(query); err != nil {
			log.Error().Err(err).Msg("error creating table")
			tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Info().Msg("Tables initialized successfully")
	return nil
}

// execQuery executes a SQL query within a transaction.
func (w *WorkspaceDB) execQuery(tx *sql.Tx, query string, args ...interface{}) error {

	if w.DB == nil {
		return fmt.Errorf("database connection is not established")
	}

	_, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}

// CommitTransaction commits a given transaction
func (w *WorkspaceDB) CommitTransaction(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}
	log.Info().Msg("Transaction committed successfully")
	return nil
}
