package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
)

// @Summary List files in a workspace
// @Description List files for object and/or block stores in a workspace.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param store query string false "Store type: object or block"
// @Success 200 {object} services.FileListResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files [get]
func GetWorkspaceFiles(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.ListFilesService(w, r)
	}
}

// @Summary Upload files to the workspace object store
// @Description Upload files to the workspace object store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param files formData file true "Files to upload"
// @Success 201 {object} services.FileUploadResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/object [post]
func UploadWorkspaceObjectFiles(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.UploadFilesService(w, r, "object")
	}
}

// @Summary Upload files to the workspace block store
// @Description Upload files to the workspace block store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param files formData file true "Files to upload"
// @Success 201 {object} services.FileUploadResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/block [post]
func UploadWorkspaceBlockFiles(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.UploadFilesService(w, r, "block")
	}
}

// @Summary Delete a file from the workspace object store
// @Description Delete a file from the workspace object store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param file query string true "File name to delete"
// @Success 200 {object} services.FileDeleteResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 409 {object} services.FileDeleteResponse
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/object [delete]
func DeleteWorkspaceObjectFile(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.DeleteFilesService(w, r, "object")
	}
}

// @Summary Delete a file from the workspace block store
// @Description Delete a file from the workspace block store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param file query string true "File name to delete"
// @Success 200 {object} services.FileDeleteResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 409 {object} services.FileDeleteResponse
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/block [delete]
func DeleteWorkspaceBlockFile(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.DeleteFilesService(w, r, "block")
	}
}

// @Summary Get object store file metadata
// @Description Get metadata for a single file in the workspace object store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param file query string true "File name within the workspace"
// @Success 200 {object} services.FileMetadataResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/object/metadata [get]
func GetWorkspaceObjectFileMetadata(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetFileMetadataService(w, r, "object")
	}
}

// @Summary Get block store file metadata
// @Description Get metadata for a single file in the workspace block store.
// @Tags Workspace Files Management
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param file query string true "File name within the workspace"
// @Success 200 {object} services.FileMetadataResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/block/metadata [get]
func GetWorkspaceBlockFileMetadata(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetFileMetadataService(w, r, "block")
	}
}

// @Summary Download a file from the workspace block store
// @Description Serves a block-store file using a short-lived signed URL (exp + sig). Clients should use the downloadUrl returned by list/metadata instead of calling this directly.
// @Tags Workspace Files Management
// @Accept json
// @Produce application/octet-stream
// @Param workspace-id path string true "Workspace ID"
// @Param file query string true "File name to download"
// @Param exp query string true "Expiry timestamp (unix)"
// @Param sig query string true "Signature"
// @Success 200 {file} file
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/files/block/download [get]
func DownloadWorkspaceBlockFile(svc *services.FileService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc.DownloadBlockFileService(w, r)
	}
}
