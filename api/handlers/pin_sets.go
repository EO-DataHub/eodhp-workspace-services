package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
)

// Trick compiler to keep import for swag annotation
var _ = ws_services.PinSet{}

// @Summary List pin sets
// @Description List the pin sets in a workspace that you can see: every workspace-wide set, plus private sets you created. Workspace admins see every private set.
// @Tags Pin Sets
// @Security BearerAuth
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Success 200 {array} ws_services.PinSetSummary
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets [get]
func ListPinSets(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.ListPinSetsService(w, r)
	}
}

// @Summary Create a pin set
// @Description Create a named pin set, optionally with items. Any workspace member can create a set. Visibility defaults to "workspace". Each item's selfHref must be on the platform STAC API. Repeated items (same selfHref) are kept once.
// @Tags Pin Sets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param body body ws_services.CreatePinSetRequest true "Pin set"
// @Success 201 {object} ws_services.PinSet
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 409 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets [post]
func CreatePinSet(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.CreatePinSetService(w, r)
	}
}

// @Summary Get a pin set
// @Description Get a pin set with its item references. Load each item from its selfHref.
// @Tags Pin Sets
// @Security BearerAuth
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Success 200 {object} ws_services.PinSet
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id} [get]
func GetPinSet(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.GetPinSetService(w, r)
	}
}

// @Summary Update a pin set
// @Description Change a pin set's name, description or visibility. Omitted fields are unchanged. Only the set's creator or a workspace admin can do this.
// @Tags Pin Sets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Param body body ws_services.UpdatePinSetRequest true "Fields to change"
// @Success 200 {object} ws_services.PinSet
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 409 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id} [patch]
func UpdatePinSet(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.UpdatePinSetService(w, r)
	}
}

// @Summary Delete a pin set
// @Description Delete a pin set and its items. Only the set's creator or a workspace admin can do this.
// @Tags Pin Sets
// @Security BearerAuth
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Success 204 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id} [delete]
func DeletePinSet(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.DeletePinSetService(w, r)
	}
}

// @Summary Replace a pin set's items
// @Description Replace every item in a pin set. Send an empty list to clear it. Only the set's creator or a workspace admin can do this.
// @Tags Pin Sets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Param body body ws_services.PinSetItemsRequest true "Items"
// @Success 200 {object} ws_services.PinSet
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id}/items [put]
func ReplacePinSetItems(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.ReplacePinSetItemsService(w, r)
	}
}

// @Summary Add items to a pin set
// @Description Add items to the end of a pin set. Items already in the set (same selfHref) are skipped. Only the set's creator or a workspace admin can do this.
// @Tags Pin Sets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Param body body ws_services.PinSetItemsRequest true "Items"
// @Success 200 {object} ws_services.PinSet
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id}/items [post]
func AddPinSetItems(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.AddPinSetItemsService(w, r)
	}
}

// @Summary Remove an item from a pin set
// @Description Remove one item from a pin set by its entry ID (the item's "id" field in the set). Only the set's creator or a workspace admin can do this.
// @Tags Pin Sets
// @Security BearerAuth
// @Param workspace-id path string true "Workspace ID"
// @Param set-id path string true "Pin set ID"
// @Param entry-id path string true "Pin set item entry ID"
// @Success 204 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/pin-sets/{set-id}/items/{entry-id} [delete]
func DeletePinSetItem(svc *services.PinSetService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.DeletePinSetItemService(w, r)
	}
}
