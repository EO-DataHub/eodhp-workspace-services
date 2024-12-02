package db

import (
	"database/sql"
	"fmt"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// GetUserWorkspaces retrieves workspaces accessible to the specified member groups.
func (db *WorkspaceDB) GetUserWorkspaces(memberGroups []string) ([]ws_manager.WorkspaceSettings, error) {

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
func (w *WorkspaceDB) CreateWorkspace(req *ws_manager.WorkspaceSettings) (*sql.Tx, error) {
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
					storeID, object.Path, object.EnvVar, object.AccessPointArn)
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
                    INSERT INTO block_stores (store_id, access_point_id, fs_id)
                    VALUES ($1, $2, $3)`,
					storeID, block.AccessPointID, block.FSID)
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

// getWorkspacesByMemberGroup retrieves workspaces for the provided member groups.
func (db *WorkspaceDB) getWorkspacesByMemberGroup(memberGroups []string) ([]ws_manager.WorkspaceSettings, error) {
	query := `SELECT id, name, account, member_group, status FROM workspaces WHERE member_group = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(memberGroups))
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []ws_manager.WorkspaceSettings
	for rows.Next() {
		var ws ws_manager.WorkspaceSettings
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.MemberGroup, &ws.Status); err != nil {
			return nil, fmt.Errorf("error scanning workspace: %w", err)
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// getWorkspacesByAccount retrieves workspaces linked to a specific account ID.
func (db *WorkspaceDB) getWorkspacesByAccount(accountID uuid.UUID) ([]ws_manager.WorkspaceSettings, error) {
	query := `SELECT id, name, account, member_group, status FROM workspaces WHERE account = $1`
	rows, err := db.DB.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []ws_manager.WorkspaceSettings
	for rows.Next() {
		var ws ws_manager.WorkspaceSettings
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Account, &ws.MemberGroup, &ws.Status); err != nil {
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
		SELECT ws.workspace_id, ws.name, bs.store_id, bs.access_point_id, bs.fs_id
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
		if err := rows.Scan(&workspaceID, &bs.Name, &bs.StoreID, &bs.AccessPointID, &bs.FSID); err != nil {
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
		SELECT ws.workspace_id, ws.name, os.store_id, os.path, os.env_var, os.access_point_arn
		FROM workspace_stores ws
		INNER JOIN object_stores os ON os.store_id = ws.id
		WHERE ws.workspace_id = ANY($1)`
	rows, err := db.DB.Query(query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("error retrieving object stores: %w", err)
	}
	defer rows.Close()

	objectStores := make(map[uuid.UUID][]ws_manager.ObjectStore)
	for rows.Next() {
		var workspaceID uuid.UUID
		var os ws_manager.ObjectStore
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
			SET access_point_id = $1, fs_id = $2
			FROM workspace_stores
			WHERE block_stores.store_id = workspace_stores.id
			  AND workspace_stores.name = $3
			  AND workspace_stores.workspace_id = $4`,
			efs.AccessPointID, efs.FSID, efs.Name, workspaceID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating block store: %w", err)
		}
	}

	// Update object_store table
	for _, bucket := range status.AWS.S3.Buckets {
		err = w.execQuery(tx, `
			UPDATE object_stores
			SET path = $1, env_var = $2, access_point_arn = $3
			FROM workspace_stores
			WHERE object_stores.store_id = workspace_stores.id
			  AND workspace_stores.name = $4
			  AND workspace_stores.workspace_id = $5`,
			bucket.Path, bucket.EnvVar, bucket.AccessPointARN, bucket.Name, workspaceID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating object store: %w", err)
		}
	}

	// Update the workspaces table status to 'created'
	err = w.execQuery(tx, `
        UPDATE workspaces
        SET status = $1
        WHERE id = $2`,
		"created", workspaceID)
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

	// Get the workspace ID from the workspace name
	fmt.Println("Workspace ID: ", workspaceID)
	return nil
}
