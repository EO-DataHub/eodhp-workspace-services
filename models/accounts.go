package models

import (
	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
)

// Account represents an account with associated workspaces.
type Account struct {
	ID           uuid.UUID                      `json:"id"`
	Name         string                         `json:"name"`
	AccountOwner string                         `json:"accountOwner"`
	Workspaces   []ws_manager.WorkspaceSettings `json:"workspaces"`
}
