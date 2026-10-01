package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// SetWorkspaceCategoryService sets a workspace's pricing category and republishes its settings
// so accounting-service picks up the change. Only a hub_admin may do this: a category sets the
// rate a workspace is charged at, so owners and workspace admins must not set their own.
func (svc *WorkspaceService) SetWorkspaceCategoryService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Workspace Scoped tokens not authorized to set a workspace category
	if claims.Workspace != "" {
		logger.Warn().Msg("Unauthorized request: workspace scoped token")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	if !HasRole(claims.RealmAccess.Roles, "hub_admin") {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied: setting workspace category")
		WriteResponse(w, http.StatusForbidden, "Access Denied: Must be a hub admin")
		return
	}

	var req models.WorkspaceCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	if !req.HasCategory() {
		logger.Warn().Msg("Invalid request payload: missing category")
		WriteResponse(w, http.StatusBadRequest, "category is required; send null to clear it")
		return
	}

	// A null category clears it, and needs no check against the configured list
	if req.Category != nil {
		categories := svc.Config.Workspaces.Categories
		if len(categories) == 0 {
			logger.Error().Msg("No workspace categories configured")
			WriteResponse(w, http.StatusServiceUnavailable, "no workspace categories are configured")
			return
		}
		if !slices.Contains(categories, *req.Category) {
			logger.Warn().Str("category", *req.Category).Msg("Invalid workspace category")
			WriteResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid category: must be one of %s, or null to clear it", strings.Join(categories, ", ")))
			return
		}
	}

	// The category is held in a transaction until the change is published, so a failed publish
	// leaves both the DB and accounting-service on the old category
	workspace, tx, err := svc.DB.SetWorkspaceCategory(workspaceID, req.Category)
	if err != nil {
		if errors.Is(err, db.ErrWorkspaceNotFound) {
			WriteResponse(w, http.StatusNotFound, "workspace not found")
			return
		}

		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Database error setting workspace category")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Republish the full workspace settings so accounting-service updates its copy of the
	// category. "updating" is used because workspace-manager consumes the same topic and rejects
	// unknown statuses; it skips the k8s write when the rebuilt spec is unchanged, as it is here.
	workspace.Status = "updating"

	if err := svc.Publisher.Publish(*workspace); err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to publish workspace category change")
		if rbErr := svc.DB.RollbackTransaction(tx); rbErr != nil {
			logger.Error().Err(rbErr).Str("workspace_id", workspaceID).Msg("Failed to roll back workspace category change")
		}
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if err := svc.DB.CommitTransaction(tx); err != nil {
		// The change was published but not saved; setting the same category again is safe
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to commit workspace category change")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logEvent := logger.Info().Str("workspace_id", workspaceID).Str("set_by", claims.Username)
	if req.Category != nil {
		logEvent.Str("category", *req.Category).Msg("Workspace category set successfully")
	} else {
		logEvent.Msg("Workspace category cleared successfully")
	}
	WriteResponse(w, http.StatusNoContent, nil)
}
