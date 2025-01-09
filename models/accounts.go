package models

import (
	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
)

// AccountsResponse holds a list of accounts.
type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
}

// AccountResponse represents a response with a single account.
type AccountResponse struct {
	Account Account `json:"account"`
}

// Account represents an account with associated workspaces.
type Account struct {
	ID           uuid.UUID                      `json:"id"`
	Name         string                         `json:"name"`
	AccountOwner string                         `json:"accountOwner"`
	Workspaces   []ws_manager.WorkspaceSettings `json:"workspaces"`
}
