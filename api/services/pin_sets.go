package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// Pin set limits.
const (
	MaxPinSetItems             = 1000
	MaxPinSetNameLength        = 100
	MaxPinSetDescriptionLength = 1000
	MaxPinSetDisplayBytes      = 1024
	maxPinSetIDLength          = 255
	maxPinSetHrefLength        = 2048
	// Large enough for MaxPinSetItems items that each use every other limit (about 3.6 MB of
	// JSON), with room for escaped characters.
	maxPinSetRequestBytes = 8 << 20
)

// PinSetService manages named sets of pinned STAC items in a workspace.
//
// Access rules: any workspace member can create a set; only its creator or a workspace admin
// can change or delete it. Private sets are visible only to their creator and admins, and
// anyone else gets a 404 so the API does not reveal that the set exists.
type PinSetService struct {
	// DB is used for the shared workspace access checks
	DB      db.WorkspaceDBInterface
	PinSets db.PinSetDBInterface
	KC      KeycloakClientInterface
	// AllowedHrefHosts are the hosts (with port, if any) that item selfHrefs can point to.
	// Other members' clients send their credentials to selfHref, so it must be the platform's
	// own STAC API and not a host chosen by whoever added the item.
	AllowedHrefHosts []string
}

// pinSetAccess is what the caller is allowed to do with the pin sets in a workspace. Admin
// status needs several DB lookups, so it is only worked out when a decision depends on it.
type pinSetAccess struct {
	workspaceMembership
	workspace string
	db        db.WorkspaceDBInterface
	admin     *bool
}

func (a *pinSetAccess) isAdmin() (bool, error) {

	if a.admin == nil {
		admin := a.superuser
		if !admin {
			var err error
			if admin, err = isUserWorkspaceAdmin(a.db, a.username, a.workspace); err != nil {
				return false, err
			}
		}
		a.admin = &admin
	}

	return *a.admin, nil
}

func (a *pinSetAccess) canSee(set *ws_services.PinSetSummary) (bool, error) {
	if set.Visibility == ws_services.PinSetVisibilityWorkspace {
		return true, nil
	}
	return a.canEdit(set)
}

func (a *pinSetAccess) canEdit(set *ws_services.PinSetSummary) (bool, error) {
	if set.CreatedBy == a.username {
		return true, nil
	}
	return a.isAdmin()
}

// ListPinSetsService lists the pin sets in a workspace that the caller can see.
func (svc *PinSetService) ListPinSetsService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, access, ok := svc.authorizeMember(w, r)
	if !ok {
		return
	}

	admin, err := access.isAdmin()
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspace).Msg("Failed to check workspace admin status")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	sets, err := svc.PinSets.ListPinSets(workspace, access.username, admin)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspace).Msg("Failed to list pin sets")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusOK, sets)
}

// CreatePinSetService creates a pin set, optionally with items.
func (svc *PinSetService) CreatePinSetService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, access, ok := svc.authorizeMember(w, r)
	if !ok {
		return
	}

	var req ws_services.CreatePinSetRequest
	if !decodePinSetBody(w, r, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Visibility == "" {
		req.Visibility = ws_services.PinSetVisibilityWorkspace
	}

	var msg string
	req.Items, msg = svc.validatePinSetItems(req.Items)
	if msg == "" {
		msg = firstNonEmpty(validatePinSetName(req.Name), validatePinSetDescription(req.Description), validatePinSetVisibility(req.Visibility))
	}
	if msg != "" {
		WriteResponse(w, http.StatusBadRequest, msg)
		return
	}

	setID, err := svc.PinSets.CreatePinSet(workspace, access.username, req)
	if err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to create pin set")
		return
	}

	set, err := svc.PinSets.GetPinSet(workspace, setID)
	if err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to retrieve created pin set")
		return
	}

	WriteResponse(w, http.StatusCreated, set, fmt.Sprintf("%s/%s", r.URL.Path, setID))
}

// GetPinSetService returns a pin set with its item references.
func (svc *PinSetService) GetPinSetService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, summary, ok := svc.authorizeSet(w, r, false)
	if !ok {
		return
	}

	// authorizeSet has already loaded the summary, so only the items are needed
	items, err := svc.PinSets.GetPinSetItems(summary.ID)
	if err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to retrieve pin set items")
		return
	}

	WriteResponse(w, http.StatusOK, ws_services.PinSet{PinSetSummary: *summary, Items: items})
}

// UpdatePinSetService changes a pin set's name, description or visibility.
func (svc *PinSetService) UpdatePinSetService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, set, ok := svc.authorizeSet(w, r, true)
	if !ok {
		return
	}
	setID := set.ID

	var req ws_services.UpdatePinSetRequest
	if !decodePinSetBody(w, r, &req) {
		return
	}

	if req.Name == nil && req.Description == nil && req.Visibility == nil {
		WriteResponse(w, http.StatusBadRequest, "Provide at least one of name, description or visibility")
		return
	}

	var msg string
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		req.Name = &name
		msg = validatePinSetName(name)
	}
	if msg == "" && req.Description != nil {
		msg = validatePinSetDescription(*req.Description)
	}
	if msg == "" && req.Visibility != nil {
		msg = validatePinSetVisibility(*req.Visibility)
	}
	if msg != "" {
		WriteResponse(w, http.StatusBadRequest, msg)
		return
	}

	if err := svc.PinSets.UpdatePinSet(workspace, setID, req); err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to update pin set")
		return
	}

	svc.writePinSet(w, logger, workspace, setID)
}

// DeletePinSetService deletes a pin set and its items.
func (svc *PinSetService) DeletePinSetService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, set, ok := svc.authorizeSet(w, r, true)
	if !ok {
		return
	}
	setID := set.ID

	if err := svc.PinSets.DeletePinSet(workspace, setID); err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to delete pin set")
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

// ReplacePinSetItemsService replaces every item in a pin set.
func (svc *PinSetService) ReplacePinSetItemsService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, set, ok := svc.authorizeSet(w, r, true)
	if !ok {
		return
	}
	setID := set.ID

	var req ws_services.PinSetItemsRequest
	if !decodePinSetBody(w, r, &req) {
		return
	}

	items, msg := svc.validatePinSetItems(req.Items)
	if msg != "" {
		WriteResponse(w, http.StatusBadRequest, msg)
		return
	}

	if err := svc.PinSets.ReplacePinSetItems(workspace, setID, items); err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to replace pin set items")
		return
	}

	svc.writePinSet(w, logger, workspace, setID)
}

// AddPinSetItemsService adds items to a pin set, skipping items already in it.
func (svc *PinSetService) AddPinSetItemsService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, set, ok := svc.authorizeSet(w, r, true)
	if !ok {
		return
	}
	setID := set.ID

	var req ws_services.PinSetItemsRequest
	if !decodePinSetBody(w, r, &req) {
		return
	}

	items, msg := svc.validatePinSetItems(req.Items)
	if msg == "" && len(items) == 0 {
		msg = "Provide at least one item"
	}
	if msg != "" {
		WriteResponse(w, http.StatusBadRequest, msg)
		return
	}

	if err := svc.PinSets.AddPinSetItems(workspace, setID, items, MaxPinSetItems); err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to add pin set items")
		return
	}

	svc.writePinSet(w, logger, workspace, setID)
}

// DeletePinSetItemService removes one item from a pin set.
func (svc *PinSetService) DeletePinSetItemService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	workspace, set, ok := svc.authorizeSet(w, r, true)
	if !ok {
		return
	}
	setID := set.ID

	entryID, err := uuid.Parse(mux.Vars(r)["entry-id"])
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, "Invalid pin set item ID")
		return
	}

	if err := svc.PinSets.DeletePinSetItem(workspace, setID, entryID); err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to remove pin set item")
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

// authorizeMember checks the caller is a member of the workspace in the URL. It writes the
// error response and returns ok=false if not.
func (svc *PinSetService) authorizeMember(w http.ResponseWriter, r *http.Request) (string, *pinSetAccess, bool) {

	logger := zerolog.Ctx(r.Context())

	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return "", nil, false
	}

	workspace := mux.Vars(r)["workspace-id"]

	membership, err := getWorkspaceMembership(svc.KC, claims, workspace)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspace).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return "", nil, false
	}

	if !membership.member {
		WriteResponse(w, http.StatusForbidden, nil)
		return "", nil, false
	}

	return workspace, &pinSetAccess{workspaceMembership: membership, workspace: workspace, db: svc.DB}, true
}

// authorizeSet checks the caller can see the pin set in the URL, and can edit it if mustEdit
// is true. It returns the set's summary, or writes the error response and returns ok=false.
func (svc *PinSetService) authorizeSet(w http.ResponseWriter, r *http.Request, mustEdit bool) (string, *ws_services.PinSetSummary, bool) {

	logger := zerolog.Ctx(r.Context())

	workspace, access, ok := svc.authorizeMember(w, r)
	if !ok {
		return "", nil, false
	}

	setID, err := uuid.Parse(mux.Vars(r)["set-id"])
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, "Invalid pin set ID")
		return "", nil, false
	}

	set, err := svc.PinSets.GetPinSetSummary(workspace, setID)
	if err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to retrieve pin set")
		return "", nil, false
	}

	canSee, err := access.canSee(set)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspace).Msg("Failed to check workspace admin status")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return "", nil, false
	}
	if !canSee {
		WriteResponse(w, http.StatusNotFound, nil)
		return "", nil, false
	}

	if mustEdit {
		canEdit, err := access.canEdit(set)
		if err != nil {
			logger.Error().Err(err).Str("workspace_id", workspace).Msg("Failed to check workspace admin status")
			WriteResponse(w, http.StatusInternalServerError, nil)
			return "", nil, false
		}
		if !canEdit {
			WriteResponse(w, http.StatusForbidden, "Only the set's creator or a workspace admin can change it")
			return "", nil, false
		}
	}

	return workspace, set, true
}

func (svc *PinSetService) writePinSet(w http.ResponseWriter, logger *zerolog.Logger, workspace string, setID uuid.UUID) {

	set, err := svc.PinSets.GetPinSet(workspace, setID)
	if err != nil {
		writePinSetDBError(w, logger, err, workspace, "Failed to retrieve pin set")
		return
	}

	WriteResponse(w, http.StatusOK, set)
}

func writePinSetDBError(w http.ResponseWriter, logger *zerolog.Logger, err error, workspace, msg string) {

	switch {
	case errors.Is(err, db.ErrPinSetNotFound), errors.Is(err, db.ErrPinSetItemNotFound), errors.Is(err, db.ErrPinSetWorkspaceNotFound):
		WriteResponse(w, http.StatusNotFound, nil)
	case errors.Is(err, db.ErrPinSetNameTaken):
		WriteResponse(w, http.StatusConflict, "A pin set with this name already exists in the workspace")
	case errors.Is(err, db.ErrPinSetTooManyItems):
		WriteResponse(w, http.StatusBadRequest, fmt.Sprintf("A pin set can hold at most %d items", MaxPinSetItems))
	default:
		logger.Error().Err(err).Str("workspace_id", workspace).Msg(msg)
		WriteResponse(w, http.StatusInternalServerError, nil)
	}
}

func decodePinSetBody(w http.ResponseWriter, r *http.Request, dest any) bool {

	r.Body = http.MaxBytesReader(w, r.Body, maxPinSetRequestBytes)

	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteResponse(w, http.StatusRequestEntityTooLarge, nil)
			return false
		}
		WriteResponse(w, http.StatusBadRequest, "Invalid request payload")
		return false
	}

	return true
}

func validatePinSetName(name string) string {
	if name == "" {
		return "Name is required"
	}
	if utf8.RuneCountInString(name) > MaxPinSetNameLength {
		return fmt.Sprintf("Name must be at most %d characters", MaxPinSetNameLength)
	}
	return ""
}

func validatePinSetDescription(description string) string {
	if utf8.RuneCountInString(description) > MaxPinSetDescriptionLength {
		return fmt.Sprintf("Description must be at most %d characters", MaxPinSetDescriptionLength)
	}
	return ""
}

func validatePinSetVisibility(visibility string) string {
	if visibility != ws_services.PinSetVisibilityWorkspace && visibility != ws_services.PinSetVisibilityPrivate {
		return fmt.Sprintf("Visibility must be %q or %q", ws_services.PinSetVisibilityWorkspace, ws_services.PinSetVisibilityPrivate)
	}
	return ""
}

// validatePinSetItems checks each item and removes repeated selfHrefs, keeping the first.
// It returns an error message, or "" if the items are valid.
func (svc *PinSetService) validatePinSetItems(items []ws_services.PinSetItemInput) ([]ws_services.PinSetItemInput, string) {

	seen := make(map[string]bool, len(items))
	unique := make([]ws_services.PinSetItemInput, 0, len(items))

	for i, item := range items {
		item.CollectionID = strings.TrimSpace(item.CollectionID)
		item.ItemID = strings.TrimSpace(item.ItemID)
		item.SelfHref = strings.TrimSpace(item.SelfHref)

		if item.CollectionID == "" || item.ItemID == "" || item.SelfHref == "" {
			return nil, fmt.Sprintf("Item %d: collectionId, itemId and selfHref are required", i)
		}
		if len(item.CollectionID) > maxPinSetIDLength || len(item.ItemID) > maxPinSetIDLength || len(item.SelfHref) > maxPinSetHrefLength {
			return nil, fmt.Sprintf("Item %d: collectionId, itemId or selfHref is too long", i)
		}

		href, err := url.Parse(item.SelfHref)
		if err != nil || (href.Scheme != "http" && href.Scheme != "https") || href.Host == "" {
			return nil, fmt.Sprintf("Item %d: selfHref must be an absolute http(s) URL", i)
		}
		if !slices.ContainsFunc(svc.AllowedHrefHosts, func(host string) bool { return strings.EqualFold(href.Host, host) }) {
			return nil, fmt.Sprintf("Item %d: selfHref must point to the platform STAC API", i)
		}

		display := bytes.TrimSpace(item.Display)
		if bytes.Equal(display, []byte("null")) {
			display = nil
		}
		if len(display) > MaxPinSetDisplayBytes {
			return nil, fmt.Sprintf("Item %d: display must be at most %d bytes", i, MaxPinSetDisplayBytes)
		}
		if len(display) > 0 && display[0] != '{' {
			return nil, fmt.Sprintf("Item %d: display must be a JSON object", i)
		}
		item.Display = display

		if seen[item.SelfHref] {
			continue
		}
		seen[item.SelfHref] = true
		unique = append(unique, item)
	}

	if len(unique) > MaxPinSetItems {
		return nil, fmt.Sprintf("A pin set can hold at most %d items", MaxPinSetItems)
	}

	return unique, ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
