package models

import (
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
	Workspaces []Workspace `json:"workspaces"`
}

// WorkspaceResponse represents a response with a single workspace.
type WorkspaceResponse struct {
	Workspace Workspace `json:"workspace"`
}

// AccountsResponse holds a list of accounts.
type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
}

// AccountResponse represents a response with a single account.
type AccountResponse struct {
	Account Account `json:"account"`
}

// ObjectStore represents an object storage entry with related metadata.
type ObjectStore struct {
	StoreID        uuid.UUID `json:"store_id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	EnvVar         string    `json:"env_var"`
	AccessPointArn string    `json:"access_point_arn"`
}

// BlockStore represents a block storage entry with related metadata.
type BlockStore struct {
	StoreID       uuid.UUID `json:"store_id"`
	Name          string    `json:"name"`
	AccessPointID string    `json:"access_point_id"`
	FSID          string    `json:"fs_id"`
}

// Stores holds lists of object and block stores associated with a workspace.
type Stores struct {
	Object []ObjectStore `json:"object"`
	Block  []BlockStore  `json:"block"`
}

// Workspace represents a workspace with associated stores.
type Workspace struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Account     uuid.UUID `json:"account"`
	MemberGroup string    `json:"member_group"`
	Status      string    `json:"status"`
	Stores      *[]Stores `json:"stores"`
}

// Account represents an account with associated workspaces.
type Account struct {
	ID           uuid.UUID   `json:"id"`
	Name         string      `json:"name"`
	AccountOwner string      `json:"accountOwner"`
	Workspaces   []Workspace `json:"workspaces"`
}
