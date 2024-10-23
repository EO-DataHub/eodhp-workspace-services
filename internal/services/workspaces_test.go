package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Implements the Notifier interface for testing
type MockEventPublisher struct {
	PublishedMessage models.ReqMessagePayload
	AckResponse      models.AckPayload
}

// Mock the Publish function to avoid hitting real external dependencies
func (m *MockEventPublisher) Publish(event models.ReqMessagePayload) error {
	m.PublishedMessage = event

	// Determine the response based on the action (status)
	switch event.Status {
	case "creating":
		// Return a full ACK response
		m.AckResponse = models.AckPayload{
			MessagePayload: event,
			AWS: models.AckAWSStatus{
				Role: models.AckAWSRoleStatus{
					Name: "WorkspaceRole",
					ARN:  "arn:aws:iam::123456789012:role/WorkspaceRole",
				},
				EFS: models.AckEFSStatus{
					AccessPoints: []models.AckEFSAccessStatus{
						{AccessPointID: "fsap-0123456789abcdef0", FSID: "fs-54321abcd", Name: "primary-efs-access-point"},
					},
				},
				S3: models.AckS3Status{
					Buckets: []models.AckS3BucketStatus{
						{Name: "test-bucket", AccessPointARN: "arn:aws:s3:accesspoint:test-region:123456789012:test-bucket-access-point", Path: "/test-bucket-path", EnvVar: "TEST_BUCKET"},
					},
				},
			},
		}
		m.AckResponse.MessagePayload.Status = "created"
	case "updated":
		// Return a full ACK response
		m.AckResponse = models.AckPayload{
			MessagePayload: event,
			AWS: models.AckAWSStatus{
				Role: models.AckAWSRoleStatus{
					Name: "UpdatedWorkspaceRole",
					ARN:  "arn:aws:iam::123456789012:role/UpdatedWorkspaceRole",
				},
				EFS: models.AckEFSStatus{
					AccessPoints: []models.AckEFSAccessStatus{
						{AccessPointID: "fsap-9876543210fedcba", FSID: "fs-98765zyxw", Name: "updated-efs-access-point"},
					},
				},
				S3: models.AckS3Status{
					Buckets: []models.AckS3BucketStatus{
						{Name: "updated-bucket", AccessPointARN: "arn:aws:s3:accesspoint:test-region:123456789012:updated-bucket-access-point", Path: "/updated-bucket-path", EnvVar: "UPDATED_BUCKET"},
					},
				},
			},
		}
		m.AckResponse.MessagePayload.Status = "updated"
	case "deleted":
		// only return the status and original event
		m.AckResponse = models.AckPayload{
			MessagePayload: event,
			AWS:            models.AckAWSStatus{}, // No AWS-related information for deletion
		}
		m.AckResponse.MessagePayload.Status = "deleted"
	default:
		// Handle unknown actions
		m.AckResponse = models.AckPayload{
			MessagePayload: event,
			AWS:            models.AckAWSStatus{},
		}
		m.AckResponse.MessagePayload.Status = "unknown"
	}

	return nil
}

// Mock the ReceiveAck function to simulate receiving an ACK response
func (m *MockEventPublisher) ReceiveAck(messagePayload models.ReqMessagePayload) (*models.AckPayload, error) {
	return &m.AckResponse, nil
}

// Mock the Close function to avoid hitting real external dependencies (simulate)
func (m *MockEventPublisher) Close() {
}

// Helper function to setup PostgreSQL container using testcontainers
func setupPostgresContainer(t *testing.T) (*sql.DB, string, func()) {
	// Request a PostgreSQL container from testcontainers
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}

	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("could not start container: %s", err)
	}

	// Get the container's host and port
	host, _ := postgresC.Host(ctx)
	port, _ := postgresC.MappedPort(ctx, "5432/tcp")

	// Form the connection string
	connStr := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())

	// Open a connection to PostgreSQL
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to open db connection: %s", err)
	}

	// Return the connection string and cleanup function
	return dbConn, connStr, func() {
		dbConn.Close()
		postgresC.Terminate(ctx)
	}
}

// Helper function to mock the JWT claims in the context
func mockJWTClaims(req *http.Request) *http.Request {
	claims := authn.Claims{
		Username: "test-owner",
	}
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, claims)
	return req.WithContext(ctx)
}

func TestAPIOperations(t *testing.T) {
	// Set up PostgreSQL container and connection string
	dbConn, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	// Initialize the mock publisher
	mockPublisher := &MockEventPublisher{}
	mockLogger := zerolog.New(os.Stdout)

	// Create the actual WorkspaceDB struct with the test database connection and mock event publisher
	mockDB := &db.WorkspaceDB{
		DB:     dbConn,        // Real database connection from Testcontainers
		Events: mockPublisher, // Mock publisher simulating event notifications and ACKs
		Log:    &mockLogger,   // Mock logger
	}

	// Initialize the tables in the database
	err := mockDB.InitTables()
	assert.NoError(t, err)

	// Run each API test - for now just the ones fully impemented
	t.Run("Test Create Workspace", func(t *testing.T) {
		testCreateWorkspace(t, mockDB)
	})

	// Add more tests here once the handlers are written
}

// Tests the creation of a workspace request, receiving the ACK, and storing data in the mock database
func testCreateWorkspace(t *testing.T, mockDB *db.WorkspaceDB) {

	// Mock a workspace request for creation
	workspaceRequest := models.ReqMessagePayload{
		Status:       "creating",
		Name:         "test-workspace",
		AccountOwner: "test-owner",
		Timestamp:    time.Now().Unix(),
	}

	// Convert the workspace request to JSON
	requestBody, err := json.Marshal(workspaceRequest)
	assert.NoError(t, err)

	// Create a new HTTP request without going through middleware
	req, err := http.NewRequest("POST", "/workspace", bytes.NewBuffer(requestBody))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Add mock claims to context
	req = mockJWTClaims(req)

	// Use httptest to record the response
	rr := httptest.NewRecorder()

	// Call the handler directly (skipping middleware)
	CreateWorkspaceService(mockDB, rr, req)

	// Check the status code
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Verify that the ACK payload is correct for creation
	ack := mockDB.Events.(*MockEventPublisher).AckResponse
	assert.Equal(t, "created", ack.MessagePayload.Status)
	assert.NotEmpty(t, ack.AWS.Role.ARN)
	assert.NotEmpty(t, ack.AWS.EFS.AccessPoints)
	assert.NotEmpty(t, ack.AWS.S3.Buckets)

	// Verify that the workspace was inserted into the database
	var workspaceCount int
	err = mockDB.DB.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE name = $1`, "test-workspace").Scan(&workspaceCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, workspaceCount)
}
