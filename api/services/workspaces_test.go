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

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Implements the Notifier interface for testing
type MockEventPublisher struct {
	PublishedMessage models.Workspace
	Response         models.Workspace
}

// Mock the Publish function to avoid hitting real external dependencies
func (m *MockEventPublisher) Publish(event models.Workspace) error {
	m.PublishedMessage = event

	// Determine the response based on the action (status)
	// Prepare the mock response based on the operation status
	switch event.Status {
	case "creating":
		// Populate a full Workspace response for "creating" status
		m.Response = models.Workspace{
			ID:          uuid.New(), // Simulate new ID
			Name:        event.Name,
			Account:     event.Account,
			MemberGroup: event.MemberGroup,
			Status:      "created",
			Stores: &[]models.Stores{
				{
					Object: []models.ObjectStore{
						{
							StoreID:        uuid.New(),
							Path:           "/test-bucket-path",
							EnvVar:         "TEST_BUCKET",
							AccessPointArn: "arn:aws:s3:accesspoint:test-region:123456789012:test-bucket-access-point",
						},
					},
					Block: []models.BlockStore{
						{
							StoreID:       uuid.New(),
							AccessPointID: "fsap-0123456789abcdef0",
							FSID:          "fs-54321abcd",
						},
					},
				},
			},
		}
	case "updated":
		// Populate a response for "updated" status
		m.Response = models.Workspace{
			ID:          event.ID,
			Name:        event.Name,
			Account:     event.Account,
			MemberGroup: event.MemberGroup,
			Status:      "updated",
			Stores: &[]models.Stores{
				{
					Object: []models.ObjectStore{
						{
							StoreID:        uuid.New(),
							Path:           "/updated-bucket-path",
							EnvVar:         "UPDATED_BUCKET",
							AccessPointArn: "arn:aws:s3:accesspoint:test-region:123456789012:updated-bucket-access-point",
						},
					},
					Block: []models.BlockStore{
						{
							StoreID:       uuid.New(),
							AccessPointID: "fsap-9876543210fedcba",
							FSID:          "fs-98765zyxw",
						},
					},
				},
			},
		}
	case "deleted":
		// Populate a response with minimal information for "deleted" status
		m.Response = models.Workspace{
			ID:          event.ID,
			Name:        event.Name,
			Account:     event.Account,
			MemberGroup: event.MemberGroup,
			Status:      "deleted",
		}
	default:
		// Populate a response for unknown status
		m.Response = models.Workspace{
			ID:          event.ID,
			Name:        event.Name,
			Account:     event.Account,
			MemberGroup: event.MemberGroup,
			Status:      "unknown",
		}
	}

	return nil
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
	workspaceRequest := models.Workspace{
		Status: "creating",
		Name:   "test-workspace",
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

	// Verify that the workspace creation event was published
	publishedMessage := mockDB.Events.(*MockEventPublisher).PublishedMessage
	assert.Equal(t, "creating", publishedMessage.Status)
	assert.Equal(t, "test-workspace", publishedMessage.Name)

	// Verify that the workspace was inserted into the database
	var workspaceCount int
	err = mockDB.DB.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE name = $1`, "test-workspace").Scan(&workspaceCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, workspaceCount)
}
