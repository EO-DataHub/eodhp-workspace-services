package services

import (
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"k8s.io/client-go/kubernetes"
)

// Service contains all shared dependencies for handlers.
type Service struct {
	Config    *appconfig.Config
	DB        *db.WorkspaceDB
	Publisher *events.EventPublisher
	KC        *KeycloakClient
}

type LinkedAccountService struct {
	DB             *db.WorkspaceDB
	SecretsManager *secretsmanager.Client
	K8sClient      *kubernetes.Clientset
}

type BillingAccountService struct {
	DB             *db.WorkspaceDB
	AWSEmailClient *sesv2.Client
}
