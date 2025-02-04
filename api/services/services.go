package services

import (
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/config"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
)

// Service contains all shared dependencies for handlers.
type Service struct {
	Config    *config.Config
	DB        *db.WorkspaceDB
	Publisher *events.EventPublisher
	KC        *KeycloakClient
}
