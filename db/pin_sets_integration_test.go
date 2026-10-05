//go:build integration

// Run against a throwaway Postgres database, e.g.:
//
//	DATABASE_URL=postgres://eodhp:eodhp@localhost:5432/pinsets_test?sslmode=disable go test -tags integration ./db/
package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newIntegrationDB(t *testing.T) (*WorkspaceDB, string) {
	t.Helper()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		t.Skip("DATABASE_URL is not set")
	}

	conn, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	wdb := &WorkspaceDB{DB: conn}
	require.NoError(t, wdb.Migrate())

	accountID := uuid.New()
	workspaceID := uuid.New()
	workspace := "ws-" + workspaceID.String()[:8]

	_, err = conn.Exec(`INSERT INTO accounts (id, name, account_owner, billing_address) VALUES ($1, 'acc', 'owner', 'addr')`, accountID)
	require.NoError(t, err)
	_, err = conn.Exec(`INSERT INTO workspaces (id, name, account, status) VALUES ($1, $2, $3, 'Available')`, workspaceID, workspace, accountID)
	require.NoError(t, err)

	t.Cleanup(func() { conn.Exec(`DELETE FROM accounts WHERE id = $1`, accountID) })

	return wdb, workspace
}

func pinItem(id string) ws_services.PinSetItemInput {
	return ws_services.PinSetItemInput{CollectionID: "s2", ItemID: id, SelfHref: "https://stac.example/items/" + id}
}

func TestPinSetLifecycle(t *testing.T) {

	wdb, workspace := newIntegrationDB(t)

	// Create with items, including display JSON
	first := pinItem("a")
	first.Display = json.RawMessage(`{"opacity": 0.5}`)
	setID, err := wdb.CreatePinSet(workspace, "alice", ws_services.CreatePinSetRequest{
		Name: "site-x", Visibility: "workspace", Items: []ws_services.PinSetItemInput{first, pinItem("b")},
	})
	require.NoError(t, err)

	set, err := wdb.GetPinSet(workspace, setID)
	require.NoError(t, err)
	require.Equal(t, 2, set.ItemCount)
	require.Equal(t, "a", set.Items[0].ItemID)
	require.JSONEq(t, `{"opacity": 0.5}`, string(set.Items[0].Display))
	require.Nil(t, set.Items[1].Display)

	// Duplicate names are rejected within a workspace
	_, err = wdb.CreatePinSet(workspace, "bob", ws_services.CreatePinSetRequest{Name: "site-x", Visibility: "workspace"})
	require.ErrorIs(t, err, ErrPinSetNameTaken)

	// Private sets are listed only for their creator or when including all private sets
	_, err = wdb.CreatePinSet(workspace, "bob", ws_services.CreatePinSetRequest{Name: "bob-private", Visibility: "private"})
	require.NoError(t, err)

	sets, err := wdb.ListPinSets(workspace, "alice", false)
	require.NoError(t, err)
	require.Len(t, sets, 1)

	sets, err = wdb.ListPinSets(workspace, "bob", false)
	require.NoError(t, err)
	require.Len(t, sets, 2)

	sets, err = wdb.ListPinSets(workspace, "admin", true)
	require.NoError(t, err)
	require.Len(t, sets, 2)

	// Adding skips items already in the set and appends after the last position
	require.NoError(t, wdb.AddPinSetItems(workspace, setID, []ws_services.PinSetItemInput{pinItem("b"), pinItem("c")}, 1000))
	set, err = wdb.GetPinSet(workspace, setID)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, itemIDs(set))

	// Adding past the limit adds nothing
	require.ErrorIs(t, wdb.AddPinSetItems(workspace, setID, []ws_services.PinSetItemInput{pinItem("d")}, 3), ErrPinSetTooManyItems)
	set, err = wdb.GetPinSet(workspace, setID)
	require.NoError(t, err)
	require.Equal(t, 3, set.ItemCount)

	// Remove one item
	require.NoError(t, wdb.DeletePinSetItem(workspace, setID, set.Items[1].ID))
	require.ErrorIs(t, wdb.DeletePinSetItem(workspace, setID, set.Items[1].ID), ErrPinSetItemNotFound)

	// Replace all items
	require.NoError(t, wdb.ReplacePinSetItems(workspace, setID, []ws_services.PinSetItemInput{pinItem("z"), pinItem("a")}))
	set, err = wdb.GetPinSet(workspace, setID)
	require.NoError(t, err)
	require.Equal(t, []string{"z", "a"}, itemIDs(set))

	// Partial update leaves other fields unchanged
	newName := "site-y"
	require.NoError(t, wdb.UpdatePinSet(workspace, setID, ws_services.UpdatePinSetRequest{Name: &newName}))
	summary, err := wdb.GetPinSetSummary(workspace, setID)
	require.NoError(t, err)
	require.Equal(t, "site-y", summary.Name)
	require.Equal(t, "workspace", summary.Visibility)

	taken := "bob-private"
	require.ErrorIs(t, wdb.UpdatePinSet(workspace, setID, ws_services.UpdatePinSetRequest{Name: &taken}), ErrPinSetNameTaken)

	// A set can't be reached through another workspace
	_, err = wdb.GetPinSetSummary("other-workspace", setID)
	require.ErrorIs(t, err, ErrPinSetNotFound)
	require.ErrorIs(t, wdb.DeletePinSet("other-workspace", setID), ErrPinSetNotFound)

	// Delete removes the set and its items
	require.NoError(t, wdb.DeletePinSet(workspace, setID))
	_, err = wdb.GetPinSet(workspace, setID)
	require.ErrorIs(t, err, ErrPinSetNotFound)

	var orphaned int
	require.NoError(t, wdb.DB.QueryRow(`SELECT COUNT(*) FROM pin_set_items WHERE set_id = $1`, setID).Scan(&orphaned))
	require.Zero(t, orphaned)
}

func itemIDs(set *ws_services.PinSet) []string {
	ids := make([]string, len(set.Items))
	for i, item := range set.Items {
		ids[i] = item.ItemID
	}
	return ids
}

func TestCreatePinSetWorkspaceNotFound(t *testing.T) {

	wdb, _ := newIntegrationDB(t)

	_, err := wdb.CreatePinSet("ws-does-not-exist", "alice", ws_services.CreatePinSetRequest{Name: "x", Visibility: "workspace"})
	require.ErrorIs(t, err, ErrPinSetWorkspaceNotFound)
}

func TestDisableWorkspaceRemovesPinSets(t *testing.T) {

	wdb, workspace := newIntegrationDB(t)

	setID, err := wdb.CreatePinSet(workspace, "alice", ws_services.CreatePinSetRequest{
		Name: "site-x", Visibility: "private", Items: []ws_services.PinSetItemInput{pinItem("a")},
	})
	require.NoError(t, err)

	require.NoError(t, wdb.DisableWorkspace(workspace))

	var sets, items int
	require.NoError(t, wdb.DB.QueryRow(`SELECT COUNT(*) FROM pin_sets WHERE id = $1`, setID).Scan(&sets))
	require.NoError(t, wdb.DB.QueryRow(`SELECT COUNT(*) FROM pin_set_items WHERE set_id = $1`, setID).Scan(&items))
	require.Zero(t, sets)
	require.Zero(t, items)
}
