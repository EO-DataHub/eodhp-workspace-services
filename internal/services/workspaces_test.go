package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MockEventPublisher implements the Notifier interface for testing
type MockEventPublisher struct{}

// Mock the Notify function to avoid hitting real external dependencies
func (m *MockEventPublisher) Notify(event events.EventPayload) error {
	// Simulate successful notification
	return nil
}

// Mock the Close function to avoid hitting real external dependencies
func (m *MockEventPublisher) Close() {
	// Simulate closing the publisher
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
	rootConnStr := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())

	// Open a connection as a superuser to PostgreSQL
	rootDB, err := sql.Open("postgres", rootConnStr)
	if err != nil {
		t.Fatalf("failed to open root db connection: %s", err)
	}

	// Ensure the "test" user and "testdb" exist
	err = setupTestUserAndDatabase(rootDB)
	if err != nil {
		t.Fatalf("failed to setup test user and database: %s", err)
	}

	// Return the connection string for the test user
	connStr := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	// Set the environment variable for the connection string
	t.Setenv("DATABASE_URL", connStr)

	// Open a connection as the "test" user
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to open test db connection: %s", err)
	}

	// Return the connection string and cleanup function
	return dbConn, connStr, func() {
		dbConn.Close()
		postgresC.Terminate(ctx)
	}
}

// setupTestUserAndDatabase ensures that the "test" user and "testdb" database exist
func setupTestUserAndDatabase(rootDB *sql.DB) error {
	// Check if the "test" user exists, and create it if it doesn't
	_, err := rootDB.Exec("DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'test') THEN CREATE ROLE test LOGIN PASSWORD 'test'; END IF; END $$;")
	if err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	// Check if the "testdb" database exists
	var dbExists bool
	err = rootDB.QueryRow("SELECT EXISTS (SELECT datname FROM pg_database WHERE datname = 'testdb')").Scan(&dbExists)
	if err != nil {
		return fmt.Errorf("failed to check if testdb exists: %w", err)
	}

	// If the database doesn't exist, create it
	if !dbExists {
		_, err = rootDB.Exec("CREATE DATABASE testdb OWNER test")
		if err != nil {
			return fmt.Errorf("failed to create testdb database: %w", err)
		}
	}

	// Grant all privileges on the database to the "test" user
	_, err = rootDB.Exec("GRANT ALL PRIVILEGES ON DATABASE testdb TO test;")
	if err != nil {
		return fmt.Errorf("failed to grant privileges to test user: %w", err)
	}

	log.Println("Test user and database setup complete")
	return nil
}

// TestCreateWorkspaceService tests the creation of a workspace and its related components
func TestCreateWorkspaceService(t *testing.T) {
	// Setup PostgreSQL container and connection string
	dbConn, _, cleanup := setupPostgresContainer(t)
	defer cleanup()

	// Mock a WorkspaceRequest object with necessary fields
	workspaceRequest := models.WorkspaceRequest{
		Name:               "test-workspace",
		Namespace:          "test-namespace",
		ServiceAccountName: "test-service-account",
		AWSRoleName:        "test-role",
		EFSAccessPoint: []models.AWSEFSAccessPoint{
			{Name: "efs-ap", FSID: "fs-123", RootDir: "/root", UID: 1001, GID: 1002, Permissions: "0755"},
		},
		S3Buckets: []models.AWSS3Bucket{
			{BucketName: "s3-bucket", BucketPath: "/path", AccessPointName: "ap-s3", EnvVar: "S3_BUCKET_VAR"},
		},
		PersistentVolumes: []models.PersistentVolume{
			{PVName: "pv-1", StorageClass: "sc1", Size: "5Gi", Driver: "gp2", AccessPointName: "ap-pv"},
		},
		PersistentVolumeClaims: []models.PersistentVolumeClaim{
			{PVCName: "pvc-1", StorageClass: "sc1", Size: "5Gi", PVName: "pv-1"},
		},
	}

	// Convert the WorkspaceRequest object to JSON to simulate a POST request
	requestBody, err := json.Marshal(workspaceRequest)
	assert.NoError(t, err)

	// Create a new HTTP request using httptest to simulate the request
	req, err := http.NewRequest("POST", "/workspace", bytes.NewBuffer(requestBody))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Use httptest to record the response
	rr := httptest.NewRecorder()

	mockPublisher := &MockEventPublisher{} // Create a mock publisher or use a real one
	mockLogger := zerolog.New(os.Stdout)   // Use a mock logger
	mockDB, err := db.NewWorkspaceDB(mockPublisher, &mockLogger)

	assert.NoError(t, err)

	// Initialize the tables
	err = mockDB.InitTables()
	assert.NoError(t, err)

	// Call the handler function directly, passing the request and response recorder
	CreateWorkspaceService(mockDB, rr, req)

	// Check that the status code is 201 Created
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Verify that the workspace and related data have been inserted into the database
	var workspaceCount int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE ws_name = $1`, "test-workspace").Scan(&workspaceCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, workspaceCount)

	// Verify AWS EFS Access Points were inserted
	var efsCount int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM efs_access_points WHERE efs_ap_name = $1`, "efs-ap").Scan(&efsCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, efsCount)

	// Verify AWS S3 Buckets were inserted
	var s3Count int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM s3_buckets WHERE s3_bucket_name = $1`, "s3-bucket").Scan(&s3Count)
	assert.NoError(t, err)
	assert.Equal(t, 1, s3Count)

	// Verify Persistent Volumes were inserted
	var pvCount int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM persistent_volumes WHERE pv_name = $1`, "pv-1").Scan(&pvCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, pvCount)

	// Verify Persistent Volume Claims were inserted
	var pvcCount int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM persistent_volume_claims WHERE pvc_name = $1`, "pvc-1").Scan(&pvcCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, pvcCount)
}
