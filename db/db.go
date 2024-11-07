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
	"github.com/lib/pq"
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

	return nil
}

func (w *WorkspaceDB) InitTables() error {

	err := w.DB.Ping()
	if err != nil {
		w.Log.Error().Err(err).Msg("Database connection ping failed")
		return fmt.Errorf("database connection ping failed: %w", err)
	}

	w.Log.Debug().Msg("Database connection is healthy, starting table initialization")

	tx, err := w.DB.Begin()
	if err != nil {
		w.Log.Error().Err(err).Msg("error starting transaction")
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Create the accounts table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS accounts (
			id UUID PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			name VARCHAR(255) NOT NULL,
			account_owner TEXT NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table accounts")

		tx.Rollback()
		return err
	}

	// Create the workspaces table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS workspaces (
			id UUID PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL,
			account_id UUID NOT NULL,
			member_group TEXT NOT NULL,
			role_name TEXT NULL,
			role_arn TEXT NULL,
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
			env_var VARCHAR(255) NOT NULL,
			access_point_arn VARCHAR(255) NOT NULL
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
			access_point_id VARCHAR(255) NOT NULL,  
			fs_id VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		w.Log.Error().Err(err).Msg("error creating table block_stores")

		tx.Rollback()
		return err
	}

	// Commit the transaction to persist changes
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	w.Log.Info().Msg("Tables initialized successfully")
	return nil
}

func (db *WorkspaceDB) GetUserWorkspaces(memberGroups []string) ([]models.Workspace, error) {

	// Get the workspaces the user is a member of
	workspaces, err := db.getWorkspaces(memberGroups)
	if err != nil {
		return nil, err
	}

	// Retrieve associated block stores for each workspace
	blockStores, err := db.getBlockStores(workspaces)
	if err != nil {
		return nil, err
	}

	// Retrieve associated object stores for each workspace
	objectStores, err := db.getObjectStores(workspaces)
	if err != nil {
		return nil, err
	}

	// Aggregate results into the response structure
	for i := range workspaces {

		workspaces[i].Stores = &[]models.Stores{
			{
				Object: objectStores[workspaces[i].ID],
				Block:  blockStores[workspaces[i].ID],
			},
		}
	}

	return workspaces, nil
}

func (db *WorkspaceDB) getWorkspaces(memberGroups []string) ([]models.Workspace, error) {
	query := `SELECT id, name, account, member_group, status FROM workspaces WHERE member_group = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(memberGroups))
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []models.Workspace
	for rows.Next() {
		var ws models.Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.MemberGroup, &ws.Status); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// Retrieves the block stores for each workspace in the provided list.
func (db *WorkspaceDB) getBlockStores(workspaces []models.Workspace) (map[uuid.UUID][]models.BlockStore, error) {
	workspaceIDs := extractWorkspaceIDs(workspaces)
	query := `
		SELECT ws.workspace_id, ws.name, bs.store_id, bs.access_point_id, bs.fs_id
		FROM workspace_stores ws
		INNER JOIN block_stores bs ON bs.store_id = ws.id
		WHERE ws.workspace_id = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("error retrieving block stores: %w", err)
	}
	defer rows.Close()

	blockStores := make(map[uuid.UUID][]models.BlockStore)
	for rows.Next() {
		var workspaceID uuid.UUID
		var bs models.BlockStore
		if err := rows.Scan(&workspaceID, &bs.Name, &bs.StoreID, &bs.AccessPointID, &bs.FSID); err != nil {
			return nil, fmt.Errorf("error scanning block store: %w", err)
		}
		blockStores[workspaceID] = append(blockStores[workspaceID], bs)
	}
	return blockStores, nil
}

// Retrieves the object stores for each workspace in the provided list.
func (db *WorkspaceDB) getObjectStores(workspaces []models.Workspace) (map[uuid.UUID][]models.ObjectStore, error) {
	workspaceIDs := extractWorkspaceIDs(workspaces)
	query := `
		SELECT ws.workspace_id, ws.name, os.store_id, os.path, os.env_var, os.access_point_arn
		FROM workspace_stores ws
		INNER JOIN object_stores os ON os.store_id = ws.id
		WHERE ws.workspace_id = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("error retrieving object stores: %w", err)
	}
	defer rows.Close()

	objectStores := make(map[uuid.UUID][]models.ObjectStore)
	for rows.Next() {
		var workspaceID uuid.UUID
		var os models.ObjectStore
		if err := rows.Scan(&workspaceID, &os.Name, &os.StoreID, &os.Path, &os.EnvVar, &os.AccessPointArn); err != nil {
			return nil, fmt.Errorf("error scanning object store: %w", err)
		}
		objectStores[workspaceID] = append(objectStores[workspaceID], os)
	}
	return objectStores, nil
}

// Helper function to extract workspace IDs for the query
func extractWorkspaceIDs(workspaces []models.Workspace) []uuid.UUID {
	ids := make([]uuid.UUID, len(workspaces))
	for i, ws := range workspaces {
		ids[i] = ws.ID
	}
	return ids
}

func (w *WorkspaceDB) InsertWorkspace(req *models.Workspace) (*sql.Tx, error) {
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Insert the workspace
	workspaceID := uuid.New() // Generate a new workspace ID
	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, name, account, member_group, status)
		VALUES ($1, $2, $3, $4, $5)`,
		workspaceID, req.Name, req.Account, req.MemberGroup, req.Status)
	if err != nil {
		w.Log.Error().Err(err).Msg("error inserting workspace")
		return nil, fmt.Errorf("error inserting workspace: %w", err)
	}

	return tx, nil
}

func (db *WorkspaceDB) GetAccounts(accountOwner string) ([]models.Account, error) {
	query := `SELECT id, name, account_owner FROM accounts WHERE account_owner = $1`
	rows, err := db.DB.Query(query, accountOwner)
	if err != nil {
		return nil, fmt.Errorf("error retrieving accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var ac models.Account
		if err := rows.Scan(&ac.ID, &ac.Name, &ac.AccountOwner); err != nil {
			return nil, fmt.Errorf("error scanning accounts: %w", err)
		}
		accounts = append(accounts, ac)
	}
	return accounts, nil
}

func (w *WorkspaceDB) InsertAccount(req *models.Account) (*models.Account, error) {

	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	accountID := uuid.New()
	created_at := time.Now().UTC()

	err = w.execQuery(tx, `
		INSERT INTO accounts (id, created_at, name, account_owner)
		VALUES ($1, $2, $3, $4)`,
		accountID, created_at, req.Name, req.AccountOwner)
	if err != nil {
		return nil, err
	}

	if err := w.CommitTransaction(tx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	account := models.Account{
		ID:           accountID,
		Name:         req.Name,
		AccountOwner: req.AccountOwner,
	}

	return &account, nil
}

func (w *WorkspaceDB) UpdateAccountOwner(accountID uuid.UUID, newOwner string) (*models.Account, error) {
	// Start a transaction
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Perform the update
	err = w.execQuery(tx, `
		UPDATE accounts 
		SET account_owner = $1 WHERE id = $2`,
		newOwner, accountID)
	if err != nil {
		tx.Rollback()
		w.Log.Error().Err(err).Msg("error updating account owner")
		return nil, fmt.Errorf("error updating account owner: %w", err)
	}

	// Commit the transaction
	if err := w.CommitTransaction(tx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	// Construct and return the updated account object
	account := &models.Account{
		ID:           accountID,
		AccountOwner: newOwner,
	}

	w.Log.Info().Msg("Account owner updated successfully")
	return account, nil
}

func (w *WorkspaceDB) DeleteAccount(accountID uuid.UUID) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Ensure the transaction is rolled back in case of an error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	err = w.execQuery(tx, `
    DELETE FROM accounts WHERE id = $1`, accountID)
	if err != nil {
		return fmt.Errorf("error executing delete query: %w", err)
	}

	if err := w.CommitTransaction(tx); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (w *WorkspaceDB) DeleteWorkspace(workspaceID uuid.UUID) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
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
		return fmt.Errorf("error committing transaction: %w", err)
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
		return err
	}
	return nil
}

func (w *WorkspaceDB) CommitTransaction(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}
	w.Log.Info().Msg("Transaction committed successfully")
	return nil
}
