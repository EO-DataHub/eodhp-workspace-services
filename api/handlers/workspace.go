package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/services"
	_ "github.com/lib/pq"
)

func CreateWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateWorkspaceService(workspaceDB, w, r)
	}
}
