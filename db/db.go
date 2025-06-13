package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// WorkspaceDBInterface defines the database operations.
type WorkspaceDBInterface interface {
	Close() error
	GetAccounts(accountOwner string) ([]ws_services.Account, error)
	GetAccount(accountID uuid.UUID) (*ws_services.Account, error)
	CreateAccount(req *ws_services.Account) (*ws_services.Account, error)
	UpdateAccount(accountID uuid.UUID, account ws_services.Account) (*ws_services.Account, error)
	DeleteAccount(accountID uuid.UUID) error
	CheckAccountIsVerified(accountID uuid.UUID) (bool, error)
	IsUserAccountOwner(username, workspaceID string) (bool, error)
	CreateAccountApprovalToken(accountID uuid.UUID) (string, error)
	ValidateApprovalToken(token string) (string, error)
	UpdateAccountStatus(token, accountID, status string) error
	GetWorkspace(workspace_name string) (*ws_manager.WorkspaceSettings, error)
	GetUserWorkspaces(memberGroups []string) ([]ws_manager.WorkspaceSettings, error)
	GetOwnedWorkspaces(username string) ([]ws_manager.WorkspaceSettings, error)
	CheckWorkspaceExists(name string) (bool, error)
	UpdateWorkspaceStatus(status ws_manager.WorkspaceStatus) error
	DisableWorkspace(workspaceName string) error
	CreateWorkspace(req *ws_manager.WorkspaceSettings) (*sql.Tx, error)
	CommitTransaction(tx *sql.Tx) error
}

// WorkspaceDB wraps database, events, and logging functionalities.
type WorkspaceDB struct {
	DB        *sql.DB
	AWSConfig *appconfig.AWSConfig
}

// Ensure WorkspaceDB implements WorkspaceDBInterface
var _ WorkspaceDBInterface = (*WorkspaceDB)(nil)

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
	return nil
}
