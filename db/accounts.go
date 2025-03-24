package db

import (
	"database/sql"
	"fmt"
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// GetAccounts retrieves all accounts owned by the specified account owner.
func (db *WorkspaceDB) GetAccounts(accountOwner string) ([]ws_services.Account, error) {
	query := `SELECT id, created_at, name, account_owner, billing_address, organization_name, account_opening_reason, status FROM accounts WHERE account_owner = $1`
	rows, err := db.DB.Query(query, accountOwner)
	if err != nil {
		return nil, fmt.Errorf("error retrieving accounts: %w", err)
	}
	defer rows.Close()

	var accounts []ws_services.Account
	for rows.Next() {
		var ac ws_services.Account
		if err := rows.Scan(&ac.ID,
			&ac.CreatedAt,
			&ac.Name,
			&ac.AccountOwner,
			&ac.BillingAddress,
			&ac.OrganizationName,
			&ac.AccountOpeningReason,
			&ac.Status); err != nil {
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
func (db *WorkspaceDB) GetAccount(accountID uuid.UUID) (*ws_services.Account, error) {
	query := `SELECT id, created_at, name, account_owner, billing_address, organization_name, account_opening_reason FROM accounts WHERE id = $1`
	row := db.DB.QueryRow(query, accountID)

	var ac ws_services.Account
	if err := row.Scan(
		&ac.ID,
		&ac.CreatedAt,
		&ac.Name,
		&ac.AccountOwner,
		&ac.BillingAddress,
		&ac.OrganizationName,
		&ac.AccountOpeningReason); err != nil {
		if err == sql.ErrNoRows {
			// Account does not exist, return nil account and nil error
			return nil, nil
		}

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
func (w *WorkspaceDB) CreateAccount(req *ws_services.Account) (*ws_services.Account, error) {

	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	accountID := uuid.New()
	created_at := time.Now().UTC()

	// Insert new account details
	err = w.execQuery(tx, `
		INSERT INTO accounts (id, created_at, name, account_owner, billing_address, organization_name, account_opening_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		accountID, created_at, req.Name, req.AccountOwner, req.BillingAddress, req.OrganizationName, req.AccountOpeningReason)
	if err != nil {
		return nil, err
	}

	// Commit transaction after insertion
	if err := w.CommitTransaction(tx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	account := ws_services.Account{
		ID:                   accountID,
		Name:                 req.Name,
		AccountOwner:         req.AccountOwner,
		BillingAddress:       req.BillingAddress,
		OrganizationName:     req.OrganizationName,
		AccountOpeningReason: req.AccountOpeningReason,
	}

	return &account, nil
}

// UpdateAccount updates an existing account's details.
func (w *WorkspaceDB) UpdateAccount(accountID uuid.UUID, account ws_services.Account) (*ws_services.Account, error) {
	// Start a transaction
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Update account fields in the database
	err = w.execQuery(tx, `
		UPDATE accounts 
		SET name = $1, account_owner = $2, billing_address = $3, organization_name = $4, account_opening_reason = $5 WHERE id = $6`,
		account.Name, account.AccountOwner, account.BillingAddress, account.OrganizationName, account.AccountOpeningReason, accountID)
	if err != nil {
		tx.Rollback()
		log.Error().Err(err).Msg("error updating account owner")
		return nil, fmt.Errorf("error updating account owner: %w", err)
	}

	// Commit transaction after update
	if err := w.CommitTransaction(tx); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	// Construct and return the updated account object
	account.ID = accountID

	log.Info().Msg("Account updated successfully")
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
func (db *WorkspaceDB) getAccountWorkspaces(accountID uuid.UUID) ([]ws_manager.WorkspaceSettings, error) {

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

// CheckAccountIsVerified checks if an account is verified and approved to use.
func (db *WorkspaceDB) CheckAccountIsVerified(accountID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 and status = 'APPROVED')`
	var exists bool
	err := db.DB.QueryRow(query, accountID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking account existence: %w", err)
	}
	return exists, nil
}

func (db *WorkspaceDB) IsUserAccountOwner(username, workspaceID string) (bool, error) {

	// Get information about the workspace
	workspace, err := db.GetWorkspace(workspaceID)

	// Check for errors
	if err != nil {
		return false, fmt.Errorf("Database error retrieving workspace: %w", err)
	}

	// Get account information
	account, err := db.GetAccount(workspace.Account)

	// Check for errors
	if err != nil {
		return false, fmt.Errorf("Database error retrieving account: %w", err)
	}

	// Check if the user is the account owner
	if username == account.AccountOwner {
		return true, nil
	}

	// Return false if the user is not the account owner
	return false, nil
}
