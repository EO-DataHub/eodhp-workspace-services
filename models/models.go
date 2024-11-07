package models

import (
	"github.com/google/uuid"
)

type Response struct {
	Success      int         `json:"success"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorDetails string      `json:"error_details,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

type WorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type WorkspaceResponse struct {
	Workspace Workspace `json:"workspace"`
}

// AccountsResponse represents a response with a list of accounts
type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
}

// AccountResponse represents a response with a single account
type AccountResponse struct {
	Account Account `json:"account"`
}


// Represents an object store entry in the database.
type ObjectStore struct {
	StoreID        uuid.UUID `json:"store_id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	EnvVar         string    `json:"env_var"`
	AccessPointArn string    `json:"access_point_arn"`
}

// BlockStore represents a block store entry in the database.
type BlockStore struct {
	StoreID       uuid.UUID `json:"store_id"`
	Name          string    `json:"name"`
	AccessPointID string    `json:"access_point_id"`
	FSID          string    `json:"fs_id"`
}

type Stores struct {
	Object []ObjectStore `json:"object"`
	Block  []BlockStore  `json:"block"`
}

type Workspace struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Account     uuid.UUID `json:"account"`
	MemberGroup string    `json:"member_group"`
	Status      string    `json:"status"`
	Stores      *[]Stores `json:"stores"`
}

type Account struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	AccountOwner string    `json:"accountOwner"`
}
