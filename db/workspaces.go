package db

import (
	"database/sql"
	"fmt"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// GetUserWorkspaces retrieves workspaces accessible to the specified member groups.
func (db *WorkspaceDB) GetUserWorkspaces(memberGroups []string) ([]models.Workspace, error) {

	// Get the workspaces the user is a member of
	workspaces, err := db.getWorkspacesByMemberGroup(memberGroups)
	if err != nil {
		return nil, err
	}

	workspaces, err = db.getWorkspaceStores(workspaces)

	if err != nil {
		return nil, err
	}

	return workspaces, nil
}

// CreateWorkspace starts a transaction to insert a new workspace record.
func (w *WorkspaceDB) CreateWorkspace(req *models.Workspace) (*sql.Tx, error) {
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Generate a new workspace ID
	workspaceID := uuid.New()

	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, name, account, member_group, status)
		VALUES ($1, $2, $3, $4, $5)`,
		workspaceID, req.Name, req.Account, req.MemberGroup, req.Status)
	if err != nil {
		return nil, fmt.Errorf("error inserting workspace: %w", err)
	}

	return tx, nil
}

// getWorkspaceStores retrieves block and object stores associated with each workspace.
func (db *WorkspaceDB) getWorkspaceStores(workspaces []models.Workspace) ([]models.Workspace, error) {

	blockStores, err := db.getBlockStores(workspaces)
	if err != nil {
		return nil, err
	}

	objectStores, err := db.getObjectStores(workspaces)
	if err != nil {
		return nil, err
	}

	// Attach stores to each workspace
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

// getWorkspacesByMemberGroup retrieves workspaces for the provided member groups.
func (db *WorkspaceDB) getWorkspacesByMemberGroup(memberGroups []string) ([]models.Workspace, error) {
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

// getWorkspacesByAccount retrieves workspaces linked to a specific account ID.
func (db *WorkspaceDB) getWorkspacesByAccount(accountID uuid.UUID) ([]models.Workspace, error) {
	query := `SELECT id, name, account, member_group, status FROM workspaces WHERE account = $1`
	rows, err := db.DB.Query(query, accountID)
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

// getBlockStores fetches block stores associated with the specified workspaces.
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

// getObjectStores fetches object stores associated with the specified workspaces.
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

// CheckWorkspaceExists checks if a workspace with the specified name already exists.
func (db *WorkspaceDB) CheckWorkspaceExists(name string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM workspaces WHERE name = $1)`
	var exists bool
	err := db.DB.QueryRow(query, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking workspace existence: %w", err)
	}
	return exists, nil
}

// extractWorkspaceIDs extracts workspace IDs from a slice of Workspace structs.
func extractWorkspaceIDs(workspaces []models.Workspace) []uuid.UUID {
	ids := make([]uuid.UUID, len(workspaces))
	for i, ws := range workspaces {
		ids[i] = ws.ID
	}
	return ids
}
