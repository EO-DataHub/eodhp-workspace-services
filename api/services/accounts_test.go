// // accounts_test.go
package services

import (
	"database/sql"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq" // Import the pq driver for PostgreSQL
	"github.com/stretchr/testify/assert"
)

// TestCreateAccount tests the creation of an account.
func TestCreateAccount(t *testing.T) {

	// Define the account to be created
	accountRequest := models.Account{
		Name:         "Test Account",
		AccountOwner: "test-owner@example.com",
	}

	// Call CreateAccount to insert the account into the database
	account, err := workspaceDB.CreateAccount(&accountRequest)
	assert.NoError(t, err, "should create account without error")
	assert.NotNil(t, account, "created account should not be nil")
	assert.Equal(t, accountRequest.Name, account.Name, "account name should match")
	assert.Equal(t, accountRequest.AccountOwner, account.AccountOwner, "account owner should match")

	// Verify that the account was inserted
	var count int
	err = workspaceDB.DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = $1`, account.ID).Scan(&count)
	assert.NoError(t, err, "should query account count without error")
	assert.Equal(t, 1, count, "account should be inserted")
}

// TestGetAccounts tests retrieving accounts for a user.
func TestGetAccounts(t *testing.T) {

	// Insert accounts
	account1 := models.Account{
		ID:             uuid.New(),
		Name:           "Account One",
		AccountOwner:   "user1@example.com",
		BillingAddress: "123 Main St",
	}
	account2 := models.Account{
		ID:             uuid.New(),
		Name:           "Account Two",
		AccountOwner:   "user1@example.com",
		BillingAddress: "456 Elm St",
	}
	account3 := models.Account{
		ID:             uuid.New(),
		Name:           "Account Three",
		AccountOwner:   "user2@example.com",
		BillingAddress: "789 Oak St",
	}

	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account1.ID, account1.Name, account1.AccountOwner, account1.BillingAddress,
	)
	assert.NoError(t, err, "should insert account one without error")

	_, err = workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account2.ID, account2.Name, account2.AccountOwner, account2.BillingAddress,
	)
	assert.NoError(t, err, "should insert account two without error")

	_, err = workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account3.ID, account3.Name, account3.AccountOwner, account2.BillingAddress,
	)
	assert.NoError(t, err, "should insert account three without error")

	// Retrieve accounts for user1@example.com
	username := "user1@example.com"
	accounts, err := workspaceDB.GetAccounts(username)
	assert.NoError(t, err, "should retrieve accounts without error")
	assert.Len(t, accounts, 2, "user1 should have two accounts")
	assert.Equal(t, "Account One", accounts[0].Name, "first account name should match")
	assert.Equal(t, "Account Two", accounts[1].Name, "second account name should match")

	// Retrieve accounts for user2@example.com
	username = "user2@example.com"
	accounts, err = workspaceDB.GetAccounts(username)
	assert.NoError(t, err, "should retrieve accounts without error")
	assert.Len(t, accounts, 1, "user2 should have one account")
	assert.Equal(t, "Account Three", accounts[0].Name, "account name should match")
}

// TestGetAccount tests retrieving a single account, including unauthorized access.
func TestGetAccount(t *testing.T) {

	// Insert an account
	accountOwner := "owner@example.com"
	account := models.Account{
		ID:             uuid.New(),
		Name:           "Owner's Account",
		AccountOwner:   accountOwner,
		BillingAddress: "123 Main St",
	}

	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account.ID, account.Name, account.AccountOwner, account.BillingAddress,
	)
	assert.NoError(t, err, "should insert account without error")

	// Test retrieving the account
	retrievedAccount, err := workspaceDB.GetAccount(account.ID)
	assert.NoError(t, err, "should retrieve account without error")
	assert.NotNil(t, retrievedAccount, "retrieved account should not be nil")
	assert.Equal(t, account.ID, retrievedAccount.ID, "account ID should match")
	assert.Equal(t, account.Name, retrievedAccount.Name, "account name should match")
	assert.Equal(t, account.AccountOwner, retrievedAccount.AccountOwner, "account owner should match")

	// Test retrieving a non-existent account
	nonExistentID := uuid.New()
	retrievedAccount, err = workspaceDB.GetAccount(nonExistentID)
	assert.Nil(t, retrievedAccount, "retrieved account should be nil for non-existent ID")
	if err != nil {
		assert.Contains(t, err.Error(), "no rows in result set", "error should indicate no rows found")
	}
}

// TestUpdateAccount tests updating an account.
func TestUpdateAccount(t *testing.T) {

	// Insert an account to be updated
	accountOwner := "owner@example.com"
	account := models.Account{
		ID:             uuid.New(),
		Name:           "Original Account Name",
		AccountOwner:   accountOwner,
		BillingAddress: "123 Main St",
	}

	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account.ID, account.Name, account.AccountOwner, account.BillingAddress,
	)
	assert.NoError(t, err, "should insert account without error")

	// Define the new name for the update
	updatedName := "Updated Account Name"

	// Update the account
	updatedAccount, err := workspaceDB.UpdateAccount(account.ID, models.Account{Name: updatedName})
	assert.NoError(t, err, "should update account without error")
	assert.NotNil(t, updatedAccount, "updated account should not be nil")
	assert.Equal(t, updatedName, updatedAccount.Name, "account name should be updated")

	// Verify that the account was updated in the database
	var retrievedName string
	err = workspaceDB.DB.QueryRow(`SELECT name FROM accounts WHERE id = $1`, account.ID).Scan(&retrievedName)
	assert.NoError(t, err, "should retrieve updated account name without error")
	assert.Equal(t, updatedName, retrievedName, "account name in DB should match updated name")

	// Test updating a non-existent account, expecting no change in the database
	nonExistentID := uuid.New()
	_, err = workspaceDB.UpdateAccount(nonExistentID, models.Account{Name: "Should Not Exist"})
	assert.NoError(t, err, "updating a non-existent account should not return an error")

	// Verify that no row was affected by checking the database for the non-existent ID
	var checkName string
	err = workspaceDB.DB.QueryRow(`SELECT name FROM accounts WHERE id = $1`, nonExistentID).Scan(&checkName)
	assert.Error(t, err, "should return an error when querying non-existent account")
	assert.Equal(t, sql.ErrNoRows, err, "should indicate no rows found for non-existent ID")
}

// TestDeleteAccount tests deleting an account.
func TestDeleteAccount(t *testing.T) {

	// Insert an account to be deleted
	accountOwner := "owner@example.com"
	account := models.Account{
		ID:             uuid.New(),
		Name:           "Account To Delete",
		AccountOwner:   accountOwner,
		BillingAddress: "123 Main St",
	}

	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner, billing_address)
		VALUES ($1, $2, $3, $4)`,
		account.ID, account.Name, account.AccountOwner, account.BillingAddress,
	)
	assert.NoError(t, err, "should insert account without error")

	// Verify the account exists in the database before deletion
	var initialCount int
	err = workspaceDB.DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = $1`, account.ID).Scan(&initialCount)
	assert.NoError(t, err, "should query initial account count without error")
	assert.Equal(t, 1, initialCount, "account should exist before deletion")

	// Delete the account
	err = workspaceDB.DeleteAccount(account.ID)
	assert.NoError(t, err, "should delete account without error")

	// Verify the account was deleted from the database
	var postDeleteCount int
	err = workspaceDB.DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = $1`, account.ID).Scan(&postDeleteCount)
	assert.NoError(t, err, "should query post-delete account count without error")
	assert.Equal(t, 0, postDeleteCount, "account should be deleted from the database")

	// Test deleting a non-existent account, expecting no error but no deletion
	nonExistentID := uuid.New()
	err = workspaceDB.DeleteAccount(nonExistentID)
	assert.NoError(t, err, "deleting a non-existent account should not return an error")

	// Verify that the non-existent account still does not exist in the database
	var checkCount int
	err = workspaceDB.DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = $1`, nonExistentID).Scan(&checkCount)
	assert.NoError(t, err, "should query count of non-existent account without error")
	assert.Equal(t, 0, checkCount, "non-existent account should not be present in the database")
}
