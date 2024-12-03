package models

import (
	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
)

// Response represents a generic API response structure.
type Response struct {
	Success      int         `json:"success"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorDetails string      `json:"error_details,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

// WorkspacesResponse holds a list of workspaces.
type WorkspacesResponse struct {
	Workspaces []ws_manager.WorkspaceSettings `json:"workspaces"`
}

// WorkspaceResponse represents a response with a single workspace.
type WorkspaceResponse struct {
	Workspace ws_manager.WorkspaceSettings `json:"workspace"`
}

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
