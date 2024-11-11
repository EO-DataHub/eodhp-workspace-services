package db

import (
	"fmt"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
)

// GetAccounts retrieves all accounts owned by the specified account owner.
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

		// Retrieve associated workspaces for the account
		workspaces, err := db.getAccountWorkspaces(ac.ID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving workspaces for account: %w", err)
		}
		ac.Workspaces = workspaces
		accounts = append(accounts, ac)
	}
	return accounts, nil
}

// GetAccount retrieves a single account.
func (db *WorkspaceDB) GetAccount(accountID uuid.UUID) (*models.Account, error) {
	query := `SELECT id, name, account_owner FROM accounts WHERE id = $1`
	row := db.DB.QueryRow(query, accountID)

	var ac models.Account
	if err := row.Scan(&ac.ID, &ac.Name, &ac.AccountOwner); err != nil {
		return nil, fmt.Errorf("error scanning accounts: %w", err)
	}

	workspaces, err := db.getAccountWorkspaces(ac.ID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces for account: %w", err)
	}
	ac.Workspaces = workspaces

	return &ac, nil

}

// CreateAccount creates a new account in the database.
func (w *WorkspaceDB) CreateAccount(req *models.Account) (*models.Account, error) {

	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	accountID := uuid.New()
	created_at := time.Now().UTC()

	// Insert new account details
	err = w.execQuery(tx, `
		INSERT INTO accounts (id, created_at, name, account_owner)
		VALUES ($1, $2, $3, $4)`,
		accountID, created_at, req.Name, req.AccountOwner)
	if err != nil {
		return nil, err
	}

	// Commit transaction after insertion
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

// UpdateAccount updates an existing account's details.
func (w *WorkspaceDB) UpdateAccount(accountID uuid.UUID, account models.Account) (*models.Account, error) {
	// Start a transaction
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Update account fields in the database
	err = w.execQuery(tx, `
		UPDATE accounts 
		SET name = $1, account_owner = $2 WHERE id = $3`,
		account.Name, account.AccountOwner, accountID)
	if err != nil {
		tx.Rollback()
		w.Log.Error().Err(err).Msg("error updating account owner")
		return nil, fmt.Errorf("error updating account owner: %w", err)
	}

	// Commit transaction after update
	if err := w.CommitTransaction(tx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	// Construct and return the updated account object
	account.ID = accountID

	w.Log.Info().Msg("Account updated successfully")
	return &account, nil
}

// DeleteAccount deletes an account from the database by its ID.
func (w *WorkspaceDB) DeleteAccount(accountID uuid.UUID) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Rollback transaction if an error occurs
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Execute delete query for the specified account ID
	err = w.execQuery(tx, `DELETE FROM accounts WHERE id = $1`, accountID)
	if err != nil {
		return fmt.Errorf("error executing delete query: %w", err)
	}

	// Commit transaction after successful deletion
	if err := w.CommitTransaction(tx); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// getAccountWorkspaces retrieves all workspaces associated with a specific account ID.
func (db *WorkspaceDB) getAccountWorkspaces(accountID uuid.UUID) ([]models.Workspace, error) {

	workspaces, err := db.getWorkspacesByAccount(accountID)
	if err != nil {
		return nil, err
	}

	// Retrieve associated stores for each workspace
	workspaces, err = db.getWorkspaceStores(workspaces)

	if err != nil {
		return nil, err
	}

	return workspaces, nil
}

// CheckAccountExists checks if an account exists in the database.
func (db *WorkspaceDB) CheckAccountExists(accountID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)`
	var exists bool
	err := db.DB.QueryRow(query, accountID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking account existence: %w", err)
	}
	return exists, nil
}
