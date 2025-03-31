package handlers

import services "github.com/EO-DataHub/eodhp-workspace-services/api/services"

const TimeFormat string = "2006-01-02T15:04:05Z"

type KeycloakClient interface {
	ExchangeToken(token, scope string) (*services.TokenResponse, error)
}