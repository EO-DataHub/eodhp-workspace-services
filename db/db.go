package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// WorkspaceDB wraps database, events, and logging functionalities.
type WorkspaceDB struct {
	DB        *sql.DB
	AWSConfig *appconfig.AWSConfig
}

// NewWorkspaceDB initializes a WorkspaceDB instance with a database connection.
func NewWorkspaceDB(awsConfig appconfig.AWSConfig) (*WorkspaceDB, error) {

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
		DB:        db,
		AWSConfig: &awsConfig,
	}, nil
}

// Close closes the database connection and event notifier.
func (w *WorkspaceDB) Close() error {
	return w.DB.Close()
}

// Migrate runs database migrations.
func (w *WorkspaceDB) Migrate() error {
	log.Info().Msg("migrating database")

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("pgx"); err != nil {
		log.Error().Err(err).Msg("failed to set database dialect")
		return err
	}

	if err := goose.Up(w.DB, "migrations"); err != nil {
		log.Error().Err(err).Msg("failed to migrate database")
		return err
	}

	log.Info().Msg("database migrated")
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
