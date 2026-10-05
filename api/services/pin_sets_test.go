package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockPinSetDB struct {
	mock.Mock
}

func (m *MockPinSetDB) ListPinSets(workspace, username string, includeAllPrivate bool) ([]ws_services.PinSetSummary, error) {
	args := m.Called(workspace, username, includeAllPrivate)
	return args.Get(0).([]ws_services.PinSetSummary), args.Error(1)
}

func (m *MockPinSetDB) GetPinSetSummary(workspace string, setID uuid.UUID) (*ws_services.PinSetSummary, error) {
	args := m.Called(workspace, setID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ws_services.PinSetSummary), args.Error(1)
}

func (m *MockPinSetDB) GetPinSet(workspace string, setID uuid.UUID) (*ws_services.PinSet, error) {
	args := m.Called(workspace, setID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ws_services.PinSet), args.Error(1)
}

func (m *MockPinSetDB) GetPinSetItems(setID uuid.UUID) ([]ws_services.PinSetItem, error) {
	args := m.Called(setID)
	return args.Get(0).([]ws_services.PinSetItem), args.Error(1)
}

func (m *MockPinSetDB) CreatePinSet(workspace, createdBy string, req ws_services.CreatePinSetRequest) (uuid.UUID, error) {
	args := m.Called(workspace, createdBy, req)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockPinSetDB) UpdatePinSet(workspace string, setID uuid.UUID, req ws_services.UpdatePinSetRequest) error {
	args := m.Called(workspace, setID, req)
	return args.Error(0)
}

func (m *MockPinSetDB) DeletePinSet(workspace string, setID uuid.UUID) error {
	args := m.Called(workspace, setID)
	return args.Error(0)
}

func (m *MockPinSetDB) ReplacePinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput) error {
	args := m.Called(workspace, setID, items)
	return args.Error(0)
}

func (m *MockPinSetDB) AddPinSetItems(workspace string, setID uuid.UUID, items []ws_services.PinSetItemInput, maxItems int) error {
	args := m.Called(workspace, setID, items, maxItems)
	return args.Error(0)
}

func (m *MockPinSetDB) DeletePinSetItem(workspace string, setID, entryID uuid.UUID) error {
	args := m.Called(workspace, setID, entryID)
	return args.Error(0)
}

const pinSetWorkspace = "ws-1"

var pinSetHrefHosts = []string{"stac.example"}

func pinSetClaims(username string) authn.Claims {
	claims := authn.Claims{Username: username}
	claims.Subject = username + "-subject"
	return claims
}

// newPinSetService returns a service where username is a workspace member, and an admin if admin is true.
func newPinSetService(username string, admin bool) (*PinSetService, *MockPinSetDB) {

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", username+"-subject").Return([]string{pinSetWorkspace}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", username, pinSetWorkspace).Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", username, pinSetWorkspace).Return(admin, nil)

	mockPinSets := new(MockPinSetDB)

	return &PinSetService{DB: mockDB, PinSets: mockPinSets, KC: mockKC, AllowedHrefHosts: pinSetHrefHosts}, mockPinSets
}

func newPinSetRequest(method string, claims authn.Claims, vars map[string]string, body string) *http.Request {

	vars["workspace-id"] = pinSetWorkspace

	req := httptest.NewRequest(method, "/workspaces/"+pinSetWorkspace+"/pin-sets", strings.NewReader(body))
	req = mux.SetURLVars(req, vars)
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, claims)
	return req.WithContext(ctx)
}

func pinSetSummary(id uuid.UUID, createdBy, visibility string) *ws_services.PinSetSummary {
	return &ws_services.PinSetSummary{ID: id, Workspace: pinSetWorkspace, Name: "site-x", Visibility: visibility, CreatedBy: createdBy}
}

func TestListPinSetsForbiddenForNonMember(t *testing.T) {
	t.Parallel()

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "outsider-subject").Return([]string{"ws-other"}, nil)
	mockPinSets := new(MockPinSetDB)
	svc := &PinSetService{DB: new(MockWorkspaceDB), PinSets: mockPinSets, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.ListPinSetsService(rec, newPinSetRequest(http.MethodGet, pinSetClaims("outsider"), map[string]string{}, ""))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockPinSets.AssertNotCalled(t, "ListPinSets", mock.Anything, mock.Anything, mock.Anything)
}

func TestListPinSetsIncludesAllPrivateForAdmin(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("admin-user", true)
	mockPinSets.On("ListPinSets", pinSetWorkspace, "admin-user", true).Return([]ws_services.PinSetSummary{}, nil)

	rec := httptest.NewRecorder()
	svc.ListPinSetsService(rec, newPinSetRequest(http.MethodGet, pinSetClaims("admin-user"), map[string]string{}, ""))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[]`, rec.Body.String())
	mockPinSets.AssertExpectations(t)
}

func TestCreatePinSetDefaultsVisibilityAndRemovesRepeatedItems(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()

	expected := ws_services.CreatePinSetRequest{
		Name:       "site-x",
		Visibility: ws_services.PinSetVisibilityWorkspace,
		Items: []ws_services.PinSetItemInput{
			{CollectionID: "s2", ItemID: "a", SelfHref: "https://stac.example/a", Display: json.RawMessage(`{"opacity":0.5}`)},
		},
	}
	mockPinSets.On("CreatePinSet", pinSetWorkspace, "member-user", expected).Return(setID, nil)
	mockPinSets.On("GetPinSet", pinSetWorkspace, setID).Return(&ws_services.PinSet{PinSetSummary: *pinSetSummary(setID, "member-user", "workspace")}, nil)

	body := `{"name":"  site-x ","items":[
		{"collectionId":"s2","itemId":"a","selfHref":"https://stac.example/a","display":{"opacity":0.5}},
		{"collectionId":"s2","itemId":"a","selfHref":"https://stac.example/a"}]}`

	rec := httptest.NewRecorder()
	svc.CreatePinSetService(rec, newPinSetRequest(http.MethodPost, pinSetClaims("member-user"), map[string]string{}, body))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), setID.String())
	mockPinSets.AssertExpectations(t)
}

func TestCreatePinSetValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"missing name":       `{"name":"  "}`,
		"long name":          `{"name":"` + strings.Repeat("a", MaxPinSetNameLength+1) + `"}`,
		"bad visibility":     `{"name":"x","visibility":"public"}`,
		"relative selfHref":  `{"name":"x","items":[{"collectionId":"c","itemId":"i","selfHref":"/items/i"}]}`,
		"selfHref off host":  `{"name":"x","items":[{"collectionId":"c","itemId":"i","selfHref":"https://attacker.example/i"}]}`,
		"selfHref userinfo":  `{"name":"x","items":[{"collectionId":"c","itemId":"i","selfHref":"https://stac.example@attacker.example/i"}]}`,
		"missing item id":    `{"name":"x","items":[{"collectionId":"c","selfHref":"https://stac.example/i"}]}`,
		"display not object": `{"name":"x","items":[{"collectionId":"c","itemId":"i","selfHref":"https://stac.example/i","display":[1]}]}`,
		"invalid json":       `{"name":`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, mockPinSets := newPinSetService("member-user", false)

			rec := httptest.NewRecorder()
			svc.CreatePinSetService(rec, newPinSetRequest(http.MethodPost, pinSetClaims("member-user"), map[string]string{}, body))

			require.Equal(t, http.StatusBadRequest, rec.Code)
			mockPinSets.AssertNotCalled(t, "CreatePinSet", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCreatePinSetNameTaken(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	mockPinSets.On("CreatePinSet", pinSetWorkspace, "member-user", mock.Anything).Return(uuid.UUID{}, db.ErrPinSetNameTaken)

	rec := httptest.NewRecorder()
	svc.CreatePinSetService(rec, newPinSetRequest(http.MethodPost, pinSetClaims("member-user"), map[string]string{}, `{"name":"site-x"}`))

	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreatePinSetWorkspaceNotFound(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	mockPinSets.On("CreatePinSet", pinSetWorkspace, "member-user", mock.Anything).Return(uuid.UUID{}, db.ErrPinSetWorkspaceNotFound)

	rec := httptest.NewRecorder()
	svc.CreatePinSetService(rec, newPinSetRequest(http.MethodPost, pinSetClaims("member-user"), map[string]string{}, `{"name":"site-x"}`))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetPinSetLoadsOnlyItemsAfterAuthorizing(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "someone-else", "workspace"), nil)
	mockPinSets.On("GetPinSetItems", setID).Return([]ws_services.PinSetItem{{CollectionID: "c", ItemID: "i"}}, nil)

	rec := httptest.NewRecorder()
	svc.GetPinSetService(rec, newPinSetRequest(http.MethodGet, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, ""))

	require.Equal(t, http.StatusOK, rec.Code)
	var set ws_services.PinSet
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &set))
	require.Equal(t, setID, set.ID)
	require.Len(t, set.Items, 1)
	mockPinSets.AssertNotCalled(t, "GetPinSet", mock.Anything, mock.Anything)
}

func TestCreatorEditsWithoutAdminLookup(t *testing.T) {
	t.Parallel()

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-user-subject").Return([]string{pinSetWorkspace}, nil)
	// No expectations: any account owner or admin lookup fails the test
	mockDB := new(MockWorkspaceDB)
	mockPinSets := new(MockPinSetDB)
	svc := &PinSetService{DB: mockDB, PinSets: mockPinSets, KC: mockKC, AllowedHrefHosts: pinSetHrefHosts}

	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "member-user", "private"), nil)
	mockPinSets.On("DeletePinSet", pinSetWorkspace, setID).Return(nil)

	rec := httptest.NewRecorder()
	svc.DeletePinSetService(rec, newPinSetRequest(http.MethodDelete, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, ""))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertNotCalled(t, "IsUserAccountOwner", mock.Anything, mock.Anything)
	mockDB.AssertNotCalled(t, "IsUserWorkspaceAdmin", mock.Anything, mock.Anything)
}

func TestGetPinSetHidesOtherUsersPrivateSet(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "someone-else", "private"), nil)

	rec := httptest.NewRecorder()
	svc.GetPinSetService(rec, newPinSetRequest(http.MethodGet, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, ""))

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockPinSets.AssertNotCalled(t, "GetPinSetItems", mock.Anything)
}

func TestGetPinSetInvalidID(t *testing.T) {
	t.Parallel()

	svc, _ := newPinSetService("member-user", false)

	rec := httptest.NewRecorder()
	svc.GetPinSetService(rec, newPinSetRequest(http.MethodGet, pinSetClaims("member-user"), map[string]string{"set-id": "not-a-uuid"}, ""))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeletePinSetForbiddenForOtherMember(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "someone-else", "workspace"), nil)

	rec := httptest.NewRecorder()
	svc.DeletePinSetService(rec, newPinSetRequest(http.MethodDelete, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, ""))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockPinSets.AssertNotCalled(t, "DeletePinSet", mock.Anything, mock.Anything)
}

func TestDeletePinSetAllowedForAdmin(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("admin-user", true)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "someone-else", "private"), nil)
	mockPinSets.On("DeletePinSet", pinSetWorkspace, setID).Return(nil)

	rec := httptest.NewRecorder()
	svc.DeletePinSetService(rec, newPinSetRequest(http.MethodDelete, pinSetClaims("admin-user"), map[string]string{"set-id": setID.String()}, ""))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockPinSets.AssertExpectations(t)
}

func TestUpdatePinSetRequiresAField(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "member-user", "workspace"), nil)

	rec := httptest.NewRecorder()
	svc.UpdatePinSetService(rec, newPinSetRequest(http.MethodPatch, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, `{}`))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPinSets.AssertNotCalled(t, "UpdatePinSet", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplacePinSetItemsRejectsTooManyItems(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "member-user", "workspace"), nil)

	items := make([]ws_services.PinSetItemInput, MaxPinSetItems+1)
	for i := range items {
		items[i] = ws_services.PinSetItemInput{CollectionID: "c", ItemID: uuid.NewString(), SelfHref: "https://stac.example/" + uuid.NewString()}
	}
	body, err := json.Marshal(ws_services.PinSetItemsRequest{Items: items})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	svc.ReplacePinSetItemsService(rec, newPinSetRequest(http.MethodPut, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, string(body)))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPinSets.AssertNotCalled(t, "ReplacePinSetItems", mock.Anything, mock.Anything, mock.Anything)
}

func TestAddPinSetItemsTooManyInSet(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "member-user", "workspace"), nil)
	mockPinSets.On("AddPinSetItems", pinSetWorkspace, setID, mock.Anything, MaxPinSetItems).Return(db.ErrPinSetTooManyItems)

	body := `{"items":[{"collectionId":"c","itemId":"i","selfHref":"https://stac.example/i"}]}`

	rec := httptest.NewRecorder()
	svc.AddPinSetItemsService(rec, newPinSetRequest(http.MethodPost, pinSetClaims("member-user"), map[string]string{"set-id": setID.String()}, body))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockPinSets.AssertExpectations(t)
}

func TestDeletePinSetItemNotFound(t *testing.T) {
	t.Parallel()

	svc, mockPinSets := newPinSetService("member-user", false)
	setID := uuid.New()
	entryID := uuid.New()
	mockPinSets.On("GetPinSetSummary", pinSetWorkspace, setID).Return(pinSetSummary(setID, "member-user", "workspace"), nil)
	mockPinSets.On("DeletePinSetItem", pinSetWorkspace, setID, entryID).Return(db.ErrPinSetItemNotFound)

	vars := map[string]string{"set-id": setID.String(), "entry-id": entryID.String()}
	rec := httptest.NewRecorder()
	svc.DeletePinSetItemService(rec, newPinSetRequest(http.MethodDelete, pinSetClaims("member-user"), vars, ""))

	require.Equal(t, http.StatusNotFound, rec.Code)
}
