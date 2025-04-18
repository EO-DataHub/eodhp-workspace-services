package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	awsclient "github.com/EO-DataHub/eodhp-workspace-services/internal/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type DataLoaderPayload struct {
	FileContent string `json:"fileContent"`
	FileName    string `json:"fileName"`
}

// AddFileDataLoader is a handler that uploads a file to S3
func AddFileDataLoader(appCfg *appconfig.Config, c STSClient, k services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		logger := zerolog.Ctx(ctx).With().Str("role arn", appCfg.AWS.S3.RoleArn).Logger()

		// Extract the workspace ID from the request URL path
		workspaceID := mux.Vars(r)["workspace-id"]

		// Parse the payload
		var payload DataLoaderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Error decoding payload")
			http.Error(w, fmt.Sprintf("Error decoding payload: %v", err), http.StatusBadRequest)
			return
		}

		bucket := appCfg.AWS.S3.Bucket

		// Create a prefix for storing eodh-config files
		objectKey := fmt.Sprintf("%s/%s/%s", workspaceID, "eodh-config", payload.FileName)

		creds, err := GetS3Credentials(appCfg.AWS.S3.RoleArn, c, k, r)
		if err != nil {
			var status int
			if httpErr, ok := err.(*services.HTTPError); ok {
				status = httpErr.Status
			} else {
				status = http.StatusInternalServerError
			}
			http.Error(w, err.Error(), status)
			return
		}

		// Create an AWS config with the temporary credentials
		cfg, err := config.LoadDefaultConfig(r.Context(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				creds.AccessKeyId,
				creds.SecretAccessKey,
				creds.SessionToken,
			)),
		)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to load AWS config")
			http.Error(w, "Failed to configure S3 client", http.StatusInternalServerError)
			return
		}

		// Create an S3 client
		s3Client := awsclient.NewS3Client(cfg)

		// Upload the file to S3
		_, err = s3Client.PutObject(r.Context(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
			Body:   bytes.NewReader([]byte(payload.FileContent)),
		})
		if err != nil {
			logger.Error().Err(err).Msg("Failed to upload file to S3")
			http.Error(w, "Failed to upload file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("File uploaded successfully to s3://%s/%s", bucket, objectKey),
		})
		logger.Info().Str("bucket", bucket).Str("key", objectKey).Msg("File uploaded to S3")

	}
}
