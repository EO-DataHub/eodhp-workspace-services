package db

import (
	"database/sql"
	"errors"
	"fmt"

	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

var (
	ErrPinSetNotFound = errors.New("pin set not found")
	// ErrPinSetWorkspaceNotFound is returned when the workspace does not exist or is deleted
	ErrPinSetWorkspaceNotFound = errors.New("workspace not found")
	ErrPinSetItemNotFound      = errors.New("pin set item not found")
	ErrPinSetNameTaken         = errors.New("a pin set with this name already exists in the workspace")
	ErrPinSetTooManyItems      = errors.New("pin set has too many items")
	uniqueViolationErrCode     = pq.ErrorCode("23505")
)

// PinSetDBInterface defines the pin set database operations. It is kept separate from
// WorkspaceDBInterface so the pin sets feature can be split out of this service later.
type PinSetDBInterface interface {
	ListPinSets(workspace, username string, includeAllPrivate bool) ([]ws_services.PinSetSummary, error)
	GetPinSetSummary(workspace string, setID uuid.UUID) (*ws_services.PinSetSummary, error)
	GetPinSet(workspace string, setID uuid.UUID) (*ws_services.PinSet, error)
	GetPinSetItems(setID uuid.UUID) ([]ws_services.PinSetItem, error)
	CreatePinSet(workspace, createdBy string, req ws_services.CreatePinSetRequest) (uuid.UUID, error)
	UpdatePinSet(workspace string, setID uuid.UUID, req ws_services.UpdatePinSetRequest) error
	DeletePinSet(workspace string, setID uuid.UUID) error
	ReplacePinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput) error
	AddPinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput, maxItems int) error
	DeletePinSetItem(workspace string, setID, entryID uuid.UUID) error
}

// Ensure WorkspaceDB implements PinSetDBInterface
var _ PinSetDBInterface = (*WorkspaceDB)(nil)

const pinSetSummaryQuery = `
	SELECT s.id, w.name, s.name, s.description, s.visibility, s.created_by, s.created_at, s.updated_at,
		(SELECT COUNT(*) FROM pin_set_items i WHERE i.set_id = s.id)
	FROM pin_sets s
	JOIN workspaces w ON w.id = s.workspace_id
	WHERE w.name = $1 AND w.status != 'Unavailable'`

// ListPinSets returns the pin sets in a workspace that the user can see: every workspace-wide
// set, plus private sets the user created, or every private set if includeAllPrivate is true.
func (db *WorkspaceDB) ListPinSets(workspace, username string, includeAllPrivate bool) ([]ws_services.PinSetSummary, error) {

	query := pinSetSummaryQuery + `
		AND (s.visibility = 'workspace' OR s.created_by = $2 OR $3)
		ORDER BY s.updated_at DESC`

	rows, err := db.DB.Query(query, workspace, username, includeAllPrivate)
	if err != nil {
		return nil, fmt.Errorf("error listing pin sets: %w", err)
	}
	defer rows.Close()

	sets := []ws_services.PinSetSummary{}
	for rows.Next() {
		set, err := scanPinSetSummary(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, *set)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error listing pin sets: %w", err)
	}

	return sets, nil
}

// GetPinSetSummary returns a pin set's details without its items.
func (db *WorkspaceDB) GetPinSetSummary(workspace string, setID uuid.UUID) (*ws_services.PinSetSummary, error) {

	row := db.DB.QueryRow(pinSetSummaryQuery+` AND s.id = $2`, workspace, setID)

	set, err := scanPinSetSummary(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPinSetNotFound
	}

	return set, err
}

// GetPinSet returns a pin set with its items in order.
func (db *WorkspaceDB) GetPinSet(workspace string, setID uuid.UUID) (*ws_services.PinSet, error) {

	summary, err := db.GetPinSetSummary(workspace, setID)
	if err != nil {
		return nil, err
	}

	items, err := db.GetPinSetItems(setID)
	if err != nil {
		return nil, err
	}

	return &ws_services.PinSet{PinSetSummary: *summary, Items: items}, nil
}

// GetPinSetItems returns a pin set's items in order. It does not check which workspace the set
// is in, so callers must have already loaded the set with GetPinSetSummary.
func (db *WorkspaceDB) GetPinSetItems(setID uuid.UUID) ([]ws_services.PinSetItem, error) {

	rows, err := db.DB.Query(`
		SELECT id, collection_id, item_id, self_href, display, position, added_at
		FROM pin_set_items
		WHERE set_id = $1
		ORDER BY position, added_at`, setID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving pin set items: %w", err)
	}
	defer rows.Close()

	items := []ws_services.PinSetItem{}
	for rows.Next() {
		var item ws_services.PinSetItem
		var display []byte
		if err := rows.Scan(&item.ID, &item.CollectionID, &item.ItemID, &item.SelfHref, &display, &item.Position, &item.AddedAt); err != nil {
			return nil, fmt.Errorf("error scanning pin set item: %w", err)
		}
		if display != nil {
			item.Display = display
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error retrieving pin set items: %w", err)
	}

	return items, nil
}

// CreatePinSet creates a pin set and its items in one transaction.
func (db *WorkspaceDB) CreatePinSet(workspace, createdBy string, req ws_services.CreatePinSetRequest) (uuid.UUID, error) {

	workspaceID, err := db.getWorkspaceID(workspace)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.UUID{}, ErrPinSetWorkspaceNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	setID := uuid.New()
	_, err = tx.Exec(`
		INSERT INTO pin_sets (id, workspace_id, name, description, visibility, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		setID, workspaceID, req.Name, req.Description, req.Visibility, createdBy)
	if err != nil {
		return uuid.UUID{}, mapPinSetWriteError(err, "error creating pin set")
	}

	if err := insertPinSetItems(tx, setID, req.Items, 0); err != nil {
		return uuid.UUID{}, err
	}

	if err := db.CommitTransaction(tx); err != nil {
		return uuid.UUID{}, err
	}

	return setID, nil
}

// UpdatePinSet changes a pin set's name, description or visibility. Nil fields are unchanged.
func (db *WorkspaceDB) UpdatePinSet(workspace string, setID uuid.UUID, req ws_services.UpdatePinSetRequest) error {

	result, err := db.DB.Exec(`
		UPDATE pin_sets s
		SET name = COALESCE($3, s.name),
			description = COALESCE($4, s.description),
			visibility = COALESCE($5, s.visibility),
			updated_at = NOW()
		FROM workspaces w
		WHERE w.id = s.workspace_id AND w.name = $1 AND s.id = $2`,
		workspace, setID, req.Name, req.Description, req.Visibility)
	if err != nil {
		return mapPinSetWriteError(err, "error updating pin set")
	}

	return requireRowsAffected(result, ErrPinSetNotFound)
}

// DeletePinSet deletes a pin set and its items. It runs in a transaction so that removing
// anything else that refers to the set (e.g. share links) can be added in the same step.
func (db *WorkspaceDB) DeletePinSet(workspace string, setID uuid.UUID) error {

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		DELETE FROM pin_sets s
		USING workspaces w
		WHERE w.id = s.workspace_id AND w.name = $1 AND s.id = $2`,
		workspace, setID)
	if err != nil {
		return fmt.Errorf("error deleting pin set: %w", err)
	}

	if err := requireRowsAffected(result, ErrPinSetNotFound); err != nil {
		return err
	}

	return db.CommitTransaction(tx)
}

// ReplacePinSetItems replaces every item in a pin set.
func (db *WorkspaceDB) ReplacePinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput) error {

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockPinSet(tx, workspace, setID); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM pin_set_items WHERE set_id = $1`, setID); err != nil {
		return fmt.Errorf("error removing pin set items: %w", err)
	}

	if err := insertPinSetItems(tx, setID, items, 0); err != nil {
		return err
	}

	if err := touchPinSet(tx, setID); err != nil {
		return err
	}

	return db.CommitTransaction(tx)
}

// AddPinSetItems appends items to a pin set, skipping items already in it. It returns
// ErrPinSetTooManyItems, and adds nothing, if the set would end up with more than maxItems.
func (db *WorkspaceDB) AddPinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput, maxItems int) error {

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockPinSet(tx, workspace, setID); err != nil {
		return err
	}

	var nextPosition int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(position) + 1, 0) FROM pin_set_items WHERE set_id = $1`, setID).Scan(&nextPosition); err != nil {
		return fmt.Errorf("error reading pin set positions: %w", err)
	}

	if err := insertPinSetItems(tx, setID, items, nextPosition); err != nil {
		return err
	}

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pin_set_items WHERE set_id = $1`, setID).Scan(&count); err != nil {
		return fmt.Errorf("error counting pin set items: %w", err)
	}
	if count > maxItems {
		return ErrPinSetTooManyItems
	}

	if err := touchPinSet(tx, setID); err != nil {
		return err
	}

	return db.CommitTransaction(tx)
}

// DeletePinSetItem removes one item from a pin set.
func (db *WorkspaceDB) DeletePinSetItem(workspace string, setID, entryID uuid.UUID) error {

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockPinSet(tx, workspace, setID); err != nil {
		return err
	}

	result, err := tx.Exec(`DELETE FROM pin_set_items WHERE id = $1 AND set_id = $2`, entryID, setID)
	if err != nil {
		return fmt.Errorf("error removing pin set item: %w", err)
	}

	if err := requireRowsAffected(result, ErrPinSetItemNotFound); err != nil {
		return err
	}

	if err := touchPinSet(tx, setID); err != nil {
		return err
	}

	return db.CommitTransaction(tx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPinSetSummary(row rowScanner) (*ws_services.PinSetSummary, error) {

	var set ws_services.PinSetSummary
	err := row.Scan(&set.ID, &set.Workspace, &set.Name, &set.Description, &set.Visibility,
		&set.CreatedBy, &set.CreatedAt, &set.UpdatedAt, &set.ItemCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning pin set: %w", err)
	}

	return &set, nil
}

// lockPinSet checks the set belongs to the workspace and locks its row, so concurrent item
// changes to the same set run one after another.
func lockPinSet(tx *sql.Tx, workspace string, setID uuid.UUID) error {

	var id uuid.UUID
	err := tx.QueryRow(`
		SELECT s.id
		FROM pin_sets s
		JOIN workspaces w ON w.id = s.workspace_id
		WHERE w.name = $1 AND s.id = $2
		FOR UPDATE OF s`, workspace, setID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPinSetNotFound
	}
	if err != nil {
		return fmt.Errorf("error locking pin set: %w", err)
	}

	return nil
}

// insertPinSetItems inserts items from startPosition onwards, skipping any self_href already in the set.
func insertPinSetItems(tx *sql.Tx, setID uuid.UUID, items []ws_services.PinSetItemInput, startPosition int) error {

	if len(items) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO pin_set_items (id, set_id, collection_id, item_id, self_href, display, position, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (set_id, self_href) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("error preparing pin set item insert: %w", err)
	}
	defer stmt.Close()

	for i, item := range items {
		// Pass display as a string: lib/pq sends []byte as bytea, which jsonb rejects
		var display any
		if len(item.Display) > 0 {
			display = string(item.Display)
		}

		if _, err := stmt.Exec(uuid.New(), setID, item.CollectionID, item.ItemID, item.SelfHref, display, startPosition+i); err != nil {
			return fmt.Errorf("error inserting pin set item: %w", err)
		}
	}

	return nil
}

func touchPinSet(tx *sql.Tx, setID uuid.UUID) error {
	if _, err := tx.Exec(`UPDATE pin_sets SET updated_at = NOW() WHERE id = $1`, setID); err != nil {
		return fmt.Errorf("error updating pin set timestamp: %w", err)
	}
	return nil
}

func requireRowsAffected(result sql.Result, notFound error) error {

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error reading affected rows: %w", err)
	}
	if affected == 0 {
		return notFound
	}

	return nil
}

func mapPinSetWriteError(err error, msg string) error {

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == uniqueViolationErrCode {
		return ErrPinSetNameTaken
	}

	return fmt.Errorf("%s: %w", msg, err)
}
