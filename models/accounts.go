package models

import (
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/google/uuid"
)

// Account represents an account with associated workspaces.
type Account struct {
	ID                   uuid.UUID                      `json:"id"`
	CreatedAt            time.Time                      `json:"createdAt"`
	Name                 string                         `json:"name"`
	AccountOwner         string                         `json:"accountOwner"`
	BillingAddress       string                         `json:"billingAddress"`
	OrganizationName     *string                        `json:"organizationName"`
	AccountOpeningReason *string                        `json:"accountOpeningReason"`
	Status               string                         `json:"status"`
	Workspaces           []ws_manager.WorkspaceSettings `json:"workspaces"`
}

type AccountStatus struct {
	Approved string
	Denied string
	Pending string
}
