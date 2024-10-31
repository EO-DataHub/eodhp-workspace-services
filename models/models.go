package models

import "github.com/google/uuid"

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"` // Message is optional; can omit if empty
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
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Account      uuid.UUID `json:"account"`
	AccountOwner string    `json:"account_owner"`
	MemberGroup  string    `json:"member_group"`
	Status       string    `json:"status"`
	Timestamp    int64     `json:"timestamp"`
	Stores       []Stores  `json:"stores"`
}
