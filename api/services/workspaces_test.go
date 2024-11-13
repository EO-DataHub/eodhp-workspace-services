package services

import (
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq" // Import the pq driver for PostgreSQL
	"github.com/stretchr/testify/assert"
)

// TestCreateWorkspace tests the creation of a workspace
func TestCreateWorkspace(t *testing.T) {
	// Create an account for testing
	accountID := uuid.New()
	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner)
		VALUES ($1, $2, $3)`,
		accountID, "Test Account", "owner@example.com",
	)
	assert.NoError(t, err, "should insert account without error")

	// Define the workspace to be created
	workspaceRequest := models.Workspace{
		ID:          uuid.New(),
		Name:        "test-workspace",
		Account:     accountID,
		MemberGroup: "test-group",
		Status:      "creating",
	}

	// Start a transaction for creating the workspace
	tx, err := workspaceDB.CreateWorkspace(&workspaceRequest)
	assert.NoError(t, err, "should start transaction without error")
	assert.NotNil(t, tx, "transaction should not be nil")

	// Simulate publishing the event
	err = workspaceDB.Events.Publish(workspaceRequest)
	assert.NoError(t, err, "should publish event without error")

	// Commit the transaction
	err = workspaceDB.CommitTransaction(tx)
	assert.NoError(t, err, "should commit transaction without error")

	// Verify that the workspace was inserted
	var count int
	err = workspaceDB.DB.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE name = $1`, workspaceRequest.Name).Scan(&count)
	assert.NoError(t, err, "should query workspace count without error")
	assert.Equal(t, 1, count, "workspace should be inserted")

	// Verify that the event was published correctly
	published := mockPublisher.PublishedMessage
	assert.Equal(t, "creating", published.Status, "published status should be 'creating'")
	assert.Equal(t, workspaceRequest.Name, published.Name, "published name should match")
}

// TestGetUserWorkspaces tests retrieving user workspaces based on member groups
func TestGetUserWorkspaces(t *testing.T) {

	// Create accounts
	accountID := uuid.New()
	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner)
		VALUES ($1, $2, $3)`,
		accountID, "Test Account", "owner@example.com",
	)
	assert.NoError(t, err, "should insert account without error")

	// Insert workspaces
	workspace1 := models.Workspace{
		ID:          uuid.New(),
		Name:        "workspace-one",
		Account:     accountID,
		MemberGroup: "group1",
		Status:      "creating",
	}
	workspace2 := models.Workspace{
		ID:          uuid.New(),
		Name:        "workspace-two",
		Account:     accountID,
		MemberGroup: "group2",
		Status:      "creating",
	}

	_, err = workspaceDB.DB.Exec(`
		INSERT INTO workspaces (id, name, account, member_group, status)
		VALUES ($1, $2, $3, $4, $5)`,
		workspace1.ID, workspace1.Name, workspace1.Account, workspace1.MemberGroup, workspace1.Status,
	)
	assert.NoError(t, err, "should insert workspace one without error")

	_, err = workspaceDB.DB.Exec(`
		INSERT INTO workspaces (id, name, account, member_group, status)
		VALUES ($1, $2, $3, $4, $5)`,
		workspace2.ID, workspace2.Name, workspace2.Account, workspace2.MemberGroup, workspace2.Status,
	)
	assert.NoError(t, err, "should insert workspace two without error")

	// Retrieve workspaces for group1
	memberGroups := []string{"group1"}
	workspaces, err := workspaceDB.GetUserWorkspaces(memberGroups)
	assert.NoError(t, err, "should retrieve workspaces without error")
	assert.Len(t, workspaces, 1, "should retrieve one workspace")
	assert.Equal(t, "workspace-one", workspaces[0].Name, "workspace name should match")

	// Retrieve workspaces for group1 and group2
	memberGroups = []string{"group1", "group2"}
	workspaces, err = workspaceDB.GetUserWorkspaces(memberGroups)
	assert.NoError(t, err, "should retrieve workspaces without error")
	assert.Len(t, workspaces, 2, "should retrieve two workspaces")
}

// TestCheckWorkspaceExists tests checking the existence of a workspace by name
func TestCheckWorkspaceExists(t *testing.T) {
	// Create an account
	accountID := uuid.New()
	_, err := workspaceDB.DB.Exec(`
		INSERT INTO accounts (id, name, account_owner)
		VALUES ($1, $2, $3)`,
		accountID, "Test Account", "owner@example.com",
	)
	assert.NoError(t, err, "should insert account without error")

	// Insert a workspace
	workspaceName := "existing-workspace"
	_, err = workspaceDB.DB.Exec(`
		INSERT INTO workspaces (id, name, account, member_group, status)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), workspaceName, accountID, "group1", "created",
	)
	assert.NoError(t, err, "should insert workspace without error")

	// Check existence of existing workspace
	exists, err := workspaceDB.CheckWorkspaceExists(workspaceName)
	assert.NoError(t, err, "should check workspace existence without error")
	assert.True(t, exists, "workspace should exist")

	// Check existence of non-existing workspace
	exists, err = workspaceDB.CheckWorkspaceExists("non-existing-workspace")
	assert.NoError(t, err, "should check workspace existence without error")
	assert.False(t, exists, "workspace should not exist")
}
