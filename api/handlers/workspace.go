package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/services"
	_ "github.com/lib/pq"
)

func CreateWorkspace() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// Call service logic to create workspace
		services.CreateWorkspaceService(w, r)
	}
}
