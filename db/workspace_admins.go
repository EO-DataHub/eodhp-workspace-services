package db

import (
	"fmt"
)

// IsUserWorkspaceAdmin checks if a user has been explicitly granted admin status on a workspace.
// It does not account for the account owner, who is an implicit admin on every workspace in
// their account - see IsUserAccountOwner.
func (db *WorkspaceDB) IsUserWorkspaceAdmin(username, workspaceID string) (bool, error) {

	workspace, err := db.GetWorkspace(workspaceID)
	if err != nil {
		return false, fmt.Errorf("database error retrieving workspace: %w", err)
	}

	query := `SELECT EXISTS(SELECT 1 FROM workspace_admins WHERE workspace_id = $1 AND username = $2)`

	var exists bool
	if err := db.DB.QueryRow(query, workspace.ID, username).Scan(&exists); err != nil {
		return false, fmt.Errorf("error checking workspace admin status: %w", err)
	}

	return exists, nil
}

// AddWorkspaceAdmin grants a workspace member admin status. It is idempotent - granting
// admin to an existing admin is a no-op.
func (db *WorkspaceDB) AddWorkspaceAdmin(username, workspaceID, addedBy string) error {

	workspace, err := db.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("database error retrieving workspace: %w", err)
	}

	query := `
	INSERT INTO workspace_admins (workspace_id, username, added_by, created_at)
	VALUES ($1, $2, $3, NOW())
	ON CONFLICT (workspace_id, username) DO NOTHING`

	if _, err := db.DB.Exec(query, workspace.ID, username, addedBy); err != nil {
		return fmt.Errorf("error adding workspace admin: %w", err)
	}

	return nil
}

// RemoveWorkspaceAdmin revokes a user's admin status on a workspace. It is a no-op if the
// user is not currently an admin.
func (db *WorkspaceDB) RemoveWorkspaceAdmin(username, workspaceID string) error {

	workspace, err := db.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("database error retrieving workspace: %w", err)
	}

	query := `DELETE FROM workspace_admins WHERE workspace_id = $1 AND username = $2`

	if _, err := db.DB.Exec(query, workspace.ID, username); err != nil {
		return fmt.Errorf("error removing workspace admin: %w", err)
	}

	return nil
}

// GetWorkspaceAdmins returns the usernames of every explicitly-granted admin on a workspace.
// It does not include the account owner, who is an implicit admin on every workspace in
// their account - see IsUserAccountOwner.
func (db *WorkspaceDB) GetWorkspaceAdmins(workspaceID string) ([]string, error) {

	workspace, err := db.GetWorkspace(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("database error retrieving workspace: %w", err)
	}

	query := `SELECT username FROM workspace_admins WHERE workspace_id = $1`

	rows, err := db.DB.Query(query, workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspace admins: %w", err)
	}
	defer rows.Close()

	admins := []string{}
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("error scanning workspace admin: %w", err)
		}
		admins = append(admins, username)
	}

	return admins, nil
}
