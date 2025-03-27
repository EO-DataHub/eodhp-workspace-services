package services

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"

// 	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
// 	"github.com/aws/aws-sdk-go-v2/service/s3"
// 	"github.com/aws/aws-sdk-go/aws"
// 	"github.com/gorilla/mux"
// 	"github.com/rs/zerolog"
// )

// type DataLoaderPayload struct {
// 	FileContent string `json:"fileContent"`
// 	FileName    string `json:"fileName"`
// }

// type DataLoaderService struct {
// 	Config *appconfig.Config
// 	KC     KeycloakClientInterface
// 	S3     *s3.Client
// }

// // AddFileDataLoaderService adds a new file to the workspaces S3 bucket
// func (svc *DataLoaderService) AddFileDataLoaderService(w http.ResponseWriter, r *http.Request) {

// 	ctx := r.Context()

// 	logger := zerolog.Ctx(ctx)

// 	// Extract the workspace ID from the request URL path
// 	workspaceID := mux.Vars(r)["workspace-id"]

// 	// Parse the payload
// 	var payload DataLoaderPayload
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Error decoding payload")
// 		WriteResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding payload: %v", err))
// 		return
// 	}

// 	bucket := "eodhp-dev-workspaces"
// 	objectKey := fmt.Sprintf("%s/%s/%s", workspaceID, "eodh-config", payload.FileName)

// 	// Upload the file to S3
// 	if err := svc.UploadFile(context.TODO(), bucket, objectKey, payload.FileContent); err != nil {
// 		logger.Error().Err(err).Str("workspace_id", workspaceID).Str("fileName", payload.FileName).Msg("Failed to upload file to S3")
// 		WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error uploading file: %v", err))
// 		return
// 	}

// 	WriteResponse(w, http.StatusCreated, nil)
// }

// // UploadFile uploads a file to S3 in the specified bucket with the given prefix.
// func (svc *DataLoaderService) UploadFile(ctx context.Context, bucket, objectKey, fileContent string) error {

// 	// Create an upload request
// 	_, err := svc.S3.PutObject(ctx, &s3.PutObjectInput{
// 		Bucket:      aws.String(bucket),
// 		Key:         aws.String(objectKey),
// 		Body:        bytes.NewReader([]byte(fileContent)),
// 		ContentType: aws.String("application/json"), // Assuming the content is JSON
// 	})

// 	return err
// }
