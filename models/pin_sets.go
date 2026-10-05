package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Pin set visibility values.
const (
	PinSetVisibilityWorkspace = "workspace"
	PinSetVisibilityPrivate   = "private"
)

// PinSetSummary describes a named set of pinned STAC items, without the items themselves.
type PinSetSummary struct {
	ID          uuid.UUID `json:"id"`
	Workspace   string    `json:"workspace"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ItemCount   int       `json:"itemCount"`
}

// PinSet is a pin set with its item references.
type PinSet struct {
	PinSetSummary
	Items []PinSetItem `json:"items"`
}

// PinSetItem is a reference to a STAC item in a pin set. Clients load the item from SelfHref,
// so the STAC API applies the reader's own permissions. SelfHref must be on a host in the
// service's allowed list (the platform STAC API), because readers send their credentials to it.
type PinSetItem struct {
	ID           uuid.UUID `json:"id"`
	CollectionID string    `json:"collectionId"`
	ItemID       string    `json:"itemId"`
	SelfHref     string    `json:"selfHref"`
	// Display holds UI-only state (render, visibility, opacity). Other clients can ignore it.
	Display  json.RawMessage `json:"display,omitempty" swaggertype:"object"`
	Position int             `json:"position"`
	AddedAt  time.Time       `json:"addedAt"`
}

// PinSetItemInput is an item reference sent by a client.
type PinSetItemInput struct {
	CollectionID string          `json:"collectionId"`
	ItemID       string          `json:"itemId"`
	SelfHref     string          `json:"selfHref"`
	Display      json.RawMessage `json:"display,omitempty" swaggertype:"object"`
}

// CreatePinSetRequest is the body for creating a pin set.
type CreatePinSetRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Visibility  string            `json:"visibility"`
	Items       []PinSetItemInput `json:"items"`
}

// UpdatePinSetRequest is the body for changing a pin set's details. Omitted fields are unchanged.
type UpdatePinSetRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
}

// PinSetItemsRequest is the body for replacing or adding pin set items.
type PinSetItemsRequest struct {
	Items []PinSetItemInput `json:"items"`
}
