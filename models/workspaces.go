package models

import ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"

// WorkspacesResponse holds a list of workspaces.
type WorkspacesResponse struct {
	Workspaces []ws_manager.WorkspaceSettings `json:"workspaces"`
}

// WorkspaceResponse represents a response with a single workspace.
type WorkspaceResponse struct {
	Workspace ws_manager.WorkspaceSettings `json:"workspace"`
}
