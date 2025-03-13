package services

import (
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Service contains all shared dependencies for handlers.
type Service struct {
	Config    *appconfig.Config
	DB        *db.WorkspaceDB
	Publisher *events.EventPublisher
	KC        *KeycloakClient
}

type LinkedAccountService struct {
	Config         *appconfig.Config
	DB             *db.WorkspaceDB
	SecretsManager *secretsmanager.Client
}
