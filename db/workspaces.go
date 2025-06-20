package db

import (
	"database/sql"
	"fmt"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// getWorkspace retrieves a workspace by name.
func (db *WorkspaceDB) GetWorkspace(workspace_name string) (*ws_manager.WorkspaceSettings, error) {

	// Check that the workspace exists
	query := `
	SELECT 
		workspaces.id, 
		workspaces.name, 
		workspaces.account, 
		accounts.account_owner as owner, 
		workspaces.status, 
		workspaces.last_updated
	FROM 
		workspaces
	INNER JOIN 
		accounts ON accounts.id = workspaces.account
	WHERE 
		workspaces.name = $1 AND workspaces.status != 'Unavailable'
	`
	rows, err := db.DB.Query(query, workspace_name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspace: %w", err)
	}
	defer rows.Close()

	var ws ws_manager.WorkspaceSettings
	if rows.Next() {
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.Owner, &ws.Status, &ws.LastUpdated); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
	} else {
		return nil, fmt.Errorf("workspace not found")
	}

	// Attach the stores to this workspace
	workspaces := []ws_manager.WorkspaceSettings{ws}
	workspacesWithStores, err := db.getWorkspaceStores(workspaces)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspace stores: %w", err)
	}

	return &workspacesWithStores[0], nil
}

// GetUserWorkspaces retrieves workspaces accessible to the specified member groups.
func (db *WorkspaceDB) GetUserWorkspaces(memberGroups []string) ([]ws_manager.WorkspaceSettings, error) {

	// Get the workspaces the user is a member of
	workspaces, err := db.getWorkspacesByGroup(memberGroups)
	if err != nil {
		return nil, err
	}

	workspaces, err = db.getWorkspaceStores(workspaces)

	if err != nil {
		return nil, err
	}

	return workspaces, nil
}

// GetOwnedWorkspaces retrieves workspaces owned by the specified username.
func (db *WorkspaceDB) GetOwnedWorkspaces(username string) ([]ws_manager.WorkspaceSettings, error) {

	// Get the workspaces the user owns
	workspaces, err := db.getWorkspacesByOwnership(username)
	if err != nil {
		return nil, err
	}

	workspaces, err = db.getWorkspaceStores(workspaces)

	if err != nil {
		return nil, err
	}

	return workspaces, nil
}

// GetAllWorkspaces retrieves all workspaces.
func (db *WorkspaceDB) GetAllWorkspaces() ([]string, error) {
	// Query to select all workspaces without filtering by member group
	query := `SELECT name FROM workspaces WHERE status != 'Unavailable'`

	// Execute the query
	rows, err := db.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	// Prepare the slice to store workspace data

	var workspaceNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error scanning workspace name: %w", err)
		}
		workspaceNames = append(workspaceNames, name)
	}

	return workspaceNames, nil
}

// CreateWorkspace starts a transaction to insert a new workspace record.
func (w *WorkspaceDB) CreateWorkspace(req *ws_manager.WorkspaceSettings) (*sql.Tx, error) {
	tx, err := w.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}

	// Generate a new workspace ID
	workspaceID := uuid.New()

	err = w.execQuery(tx, `
		INSERT INTO workspaces (id, name, account, status, last_updated)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)`,
		workspaceID, req.Name, req.Account, req.Status)
	if err != nil {
		return nil, fmt.Errorf("error inserting workspace: %w", err)
	}

	// Insert into workspace_stores and then into object_stores/block_stores
	if req.Stores != nil {
		for _, store := range *req.Stores {

			// Insert Object Stores
			for _, object := range store.Object {
				storeID := uuid.New()

				// Insert into `workspace_stores`
				err = w.execQuery(tx, `
                    INSERT INTO workspace_stores (id, workspace_id, store_type, name)
                    VALUES ($1, $2, $3, $4)`,
					storeID, workspaceID, "object", object.Name)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("error inserting into workspace_stores (object): %w", err)
				}

				// Insert into `object_stores` using the generated store ID
				err = w.execQuery(tx, `
                    INSERT INTO object_stores (store_id, path, env_var, access_point_arn)
                    VALUES ($1, $2, $3, $4)`,
					storeID, object.Prefix, object.EnvVar, object.AccessPointArn)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("error inserting into object_stores: %w", err)
				}
			}

			// Insert Block Stores
			for _, block := range store.Block {
				storeID := uuid.New()

				// Insert into `workspace_stores`
				err = w.execQuery(tx, `
                    INSERT INTO workspace_stores (id, workspace_id, store_type, name)
                    VALUES ($1, $2, $3, $4)`,
					storeID, workspaceID, "block", block.Name)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("error inserting into workspace_stores (block): %w", err)
				}

				// Insert into `block_stores` using the generated store ID
				err = w.execQuery(tx, `
                    INSERT INTO block_stores (store_id, access_point_id, mount_point)
                    VALUES ($1, $2, $3)`,
					storeID, block.AccessPointID, block.MountPoint)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("error inserting into block_stores: %w", err)
				}
			}
		}
	}

	return tx, nil
}

// getWorkspaceStores retrieves block and object stores associated with each workspace.
func (db *WorkspaceDB) getWorkspaceStores(workspaces []ws_manager.WorkspaceSettings) ([]ws_manager.WorkspaceSettings, error) {

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

		workspaces[i].Stores = &[]ws_manager.Stores{
			{
				Object: objectStores[workspaces[i].ID],
				Block:  blockStores[workspaces[i].ID],
			},
		}
	}

	return workspaces, nil

}

// getWorkspacesByGroup retrieves workspaces for the provided keycloak groups.
func (db *WorkspaceDB) getWorkspacesByGroup(memberGroups []string) ([]ws_manager.WorkspaceSettings, error) {

	query := `
	SELECT 
		workspaces.id, 
		workspaces.name, 
		workspaces.account, 
		accounts.account_owner as owner, 
		workspaces.status, 
		workspaces.last_updated
	FROM 
		workspaces
	INNER JOIN 
		accounts ON accounts.id = workspaces.account
	WHERE 
		workspaces.name = ANY($1) AND workspaces.status != 'Unavailable'
	`

	rows, err := db.DB.Query(query, pq.Array(memberGroups))
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []ws_manager.WorkspaceSettings
	for rows.Next() {
		var ws ws_manager.WorkspaceSettings
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.Owner, &ws.Status, &ws.LastUpdated); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// getWorkspacesByAccount retrieves workspaces linked to a specific account ID.
func (db *WorkspaceDB) getWorkspacesByAccount(accountID uuid.UUID) ([]ws_manager.WorkspaceSettings, error) {

	query := `
	SELECT 
		workspaces.id, 
		workspaces.name, 
		workspaces.account, 
		accounts.account_owner as owner, 
		workspaces.status, 
		workspaces.last_updated
	FROM 
		workspaces
	INNER JOIN 
		accounts ON accounts.id = workspaces.account
	WHERE 
		workspaces.account = $1 AND workspaces.status != 'Unavailable'
	`

	rows, err := db.DB.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []ws_manager.WorkspaceSettings
	for rows.Next() {
		var ws ws_manager.WorkspaceSettings
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.Owner, &ws.Status, &ws.LastUpdated); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// getWorkspacesByOwnership retrieves workspaces owned by the specified username.
func (db *WorkspaceDB) getWorkspacesByOwnership(username string) ([]ws_manager.WorkspaceSettings, error) {

	query := `
	SELECT 
		workspaces.id, 
		workspaces.name, 
		workspaces.account, 
		accounts.account_owner as owner, 
		workspaces.status, 
		workspaces.last_updated
	FROM 
		workspaces
	INNER JOIN 
		accounts ON accounts.id = workspaces.account
	WHERE 
		accounts.account_owner = $1 AND workspaces.status != 'Unavailable'
	`

	rows, err := db.DB.Query(query, username)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []ws_manager.WorkspaceSettings
	for rows.Next() {
		var ws ws_manager.WorkspaceSettings
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.Owner, &ws.Status, &ws.LastUpdated); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// getBlockStores fetches block stores associated with the specified workspaces.
func (db *WorkspaceDB) getBlockStores(workspaces []ws_manager.WorkspaceSettings) (map[uuid.UUID][]ws_manager.BlockStore, error) {
	workspaceIDs := extractWorkspaceIDs(workspaces)
	query := `
		SELECT ws.workspace_id, ws.name, bs.store_id, bs.access_point_id, bs.mount_point
		FROM workspace_stores ws
		INNER JOIN block_stores bs ON bs.store_id = ws.id
		WHERE ws.workspace_id = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("error retrieving block stores: %w", err)
	}
	defer rows.Close()

	blockStores := make(map[uuid.UUID][]ws_manager.BlockStore)
	for rows.Next() {
		var workspaceID uuid.UUID
		var bs ws_manager.BlockStore
		if err := rows.Scan(&workspaceID, &bs.Name, &bs.StoreID, &bs.AccessPointID, &bs.MountPoint); err != nil {
			return nil, fmt.Errorf("error scanning block store: %w", err)
		}
		blockStores[workspaceID] = append(blockStores[workspaceID], bs)
	}
	return blockStores, nil
}

// getObjectStores fetches object stores associated with the specified workspaces.
func (db *WorkspaceDB) getObjectStores(workspaces []ws_manager.WorkspaceSettings) (map[uuid.UUID][]ws_manager.ObjectStore, error) {
	workspaceIDs := extractWorkspaceIDs(workspaces)
	query := `
	   SELECT ws.id, ws.name, wss.name, os.* from workspaces ws
       INNER join workspace_stores wss on wss.workspace_id = ws.id
       INNER join object_stores os on os.store_id = wss.id
       WHERE ws.id = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("error retrieving object stores: %w", err)
	}
	defer rows.Close()

	objectStores := make(map[uuid.UUID][]ws_manager.ObjectStore)

	for rows.Next() {

		var workspaceName string
		var workspaceID uuid.UUID
		var os ws_manager.ObjectStore

		if err := rows.Scan(&workspaceID, &workspaceName, &os.Name, &os.StoreID, &os.Prefix, &os.EnvVar, &os.AccessPointArn); err != nil {
			return nil, fmt.Errorf("error scanning object store: %w", err)
		}

		// Derived data
		os.Bucket = db.AWSConfig.S3.Bucket
		accessPointName := func() string {
			parts := strings.Split(os.AccessPointArn, "/")
			if len(parts) > 1 {
				return parts[1]
			}
			return ""
		}()
		os.Host = fmt.Sprintf("%s-%s.%s", accessPointName, db.AWSConfig.Account, db.AWSConfig.S3.Host)
		os.AccessURL = fmt.Sprintf("https://%s.%s/files/%s/", workspaceName, db.AWSConfig.WorkspaceDomain, os.Bucket)

		// Add object store to the map
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
func extractWorkspaceIDs(workspaces []ws_manager.WorkspaceSettings) []uuid.UUID {
	ids := make([]uuid.UUID, len(workspaces))
	for i, ws := range workspaces {
		ids[i] = ws.ID
	}
	return ids
}

func (w *WorkspaceDB) UpdateWorkspaceStatus(status ws_manager.WorkspaceStatus) error {

	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Get the workspace ID from the workspaces table
	var workspaceID uuid.UUID
	err = tx.QueryRow(`SELECT id FROM workspaces WHERE name = $1`, status.Name).Scan(&workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error retrieving workspace ID: %w", err)
	}

	// Update block_store table
	for _, efs := range status.AWS.EFS.AccessPoints {
		err = w.execQuery(tx, `
			UPDATE block_stores
			SET access_point_id = $1, mount_point = $2
			FROM workspace_stores
			WHERE block_stores.store_id = workspace_stores.id
			  AND workspace_stores.name = $3
			  AND workspace_stores.workspace_id = $4`,
			efs.AccessPointID, efs.RootDirectory, efs.Name, workspaceID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating block store: %w", err)
		}
	}

	// Update object_store table
	for _, bucket := range status.AWS.S3.Buckets {

		// Extract the top-level directory (store name) from the bucket path.
		// The store name is assumed to be the first segment before the "/".
		storeName := strings.Split(bucket.Path, "/")[0]

		err = w.execQuery(tx, `
			UPDATE object_stores
			SET path = $1, env_var = $2, access_point_arn = $3
			FROM workspace_stores
			WHERE object_stores.store_id = workspace_stores.id
			AND workspace_stores.name = $4
			  AND workspace_stores.workspace_id = $5`,
			bucket.Path, bucket.EnvVar, bucket.AccessPointARN, storeName, workspaceID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating object store: %w", err)
		}

	}

	// Update the workspaces table
	err = w.execQuery(tx, `
        UPDATE workspaces
        SET role_name = $1, role_arn = $2, status = $3, last_updated = CURRENT_TIMESTAMP
        WHERE id = $4`,
		status.AWS.Role.Name, status.AWS.Role.ARN, status.State, workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error updating workspace status: %w", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (w *WorkspaceDB) DisableWorkspace(workspaceName string) error {
	tx, err := w.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Get workspace ID from name
	var workspaceID string
	err = tx.QueryRow(`SELECT id FROM workspaces WHERE name = $1`, workspaceName).Scan(&workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("workspace not found: %w", err)
	}

	// Delete from dependent tables in reverse dependency order
	_, err = tx.Exec(`DELETE FROM object_stores WHERE store_id IN 
		(SELECT id FROM workspace_stores WHERE workspace_id = $1)`, workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error deleting from object_stores: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM block_stores WHERE store_id IN 
		(SELECT id FROM workspace_stores WHERE workspace_id = $1)`, workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error deleting from block_stores: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM workspace_stores WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error deleting from workspace_stores: %w", err)
	}

	// Set workspace status to 'Unavailable'
	_, err = tx.Exec(`UPDATE workspaces SET status = 'Unavailable', role_name = NULL, role_arn = NULL, last_updated = CURRENT_TIMESTAMP WHERE id = $1`, workspaceID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error updating workspace status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}
