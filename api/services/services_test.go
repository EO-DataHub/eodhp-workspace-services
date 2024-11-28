package services

// import (
// 	"context"
// 	"database/sql"
// 	"fmt"
// 	"os"
// 	"testing"

// 	"github.com/EO-DataHub/eodhp-workspace-services/db"
// 	"github.com/EO-DataHub/eodhp-workspace-services/models"
// 	"github.com/google/uuid"
// 	"github.com/rs/zerolog"
// 	"github.com/testcontainers/testcontainers-go"
// 	"github.com/testcontainers/testcontainers-go/wait"
// )

// // MockEventPublisher implements the Notifier interface for testing
// type MockEventPublisher struct {
// 	PublishedMessage models.Workspace
// 	Response         models.Workspace
// }

// func (m *MockEventPublisher) Publish(event models.Workspace) error {
// 	m.PublishedMessage = event
// 	// Simulate different responses based on event status
// 	switch event.Status {
// 	case "creating":
// 		m.Response = models.Workspace{
// 			ID:          uuid.New(),
// 			Name:        event.Name,
// 			Account:     event.Account,
// 			MemberGroup: event.MemberGroup,
// 			Status:      "created",
// 		}
// 	case "updating":
// 		m.Response = models.Workspace{
// 			ID:          event.ID,
// 			Name:        event.Name,
// 			Account:     event.Account,
// 			MemberGroup: event.MemberGroup,
// 			Status:      "updated",
// 		}
// 	case "deleting":
// 		m.Response = models.Workspace{
// 			ID:          event.ID,
// 			Name:        event.Name,
// 			Account:     event.Account,
// 			MemberGroup: event.MemberGroup,
// 			Status:      "deleted",
// 		}
// 	default:
// 		m.Response = models.Workspace{
// 			ID:          event.ID,
// 			Name:        event.Name,
// 			Account:     event.Account,
// 			MemberGroup: event.MemberGroup,
// 			Status:      "unknown",
// 		}
// 	}
// 	return nil
// }

// func (m *MockEventPublisher) Close() {}

// var (
// 	sharedDB      *sql.DB             // shared database connection
// 	workspaceDB   *db.WorkspaceDB     // shared WorkspaceDB instance
// 	cleanupDB     func()              // function to clean up the container
// 	mockLogger    zerolog.Logger      // shared logger instance
// 	mockPublisher *MockEventPublisher // shared mock event publisher
// )

// // setupPostgresContainer initializes a PostgreSQL container for testing
// func setupPostgresContainer() (*sql.DB, func(), error) {
// 	ctx := context.Background()
// 	req := testcontainers.ContainerRequest{
// 		Image:        "postgres:13",
// 		ExposedPorts: []string{"5432/tcp"},
// 		Env: map[string]string{
// 			"POSTGRES_USER":     "postgres",
// 			"POSTGRES_PASSWORD": "postgres",
// 			"POSTGRES_DB":       "postgres",
// 		},
// 		WaitingFor: wait.ForListeningPort("5432/tcp"),
// 	}

// 	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
// 		ContainerRequest: req,
// 		Started:          true,
// 	})
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("could not start container: %w", err)
// 	}

// 	host, err := postgresC.Host(ctx)
// 	if err != nil {
// 		postgresC.Terminate(ctx)
// 		return nil, nil, fmt.Errorf("could not get container host: %w", err)
// 	}
// 	port, err := postgresC.MappedPort(ctx, "5432/tcp")
// 	if err != nil {
// 		postgresC.Terminate(ctx)
// 		return nil, nil, fmt.Errorf("could not get mapped port: %w", err)
// 	}

// 	connStr := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())

// 	// Connect to the PostgreSQL container
// 	dbConn, err := sql.Open("postgres", connStr)
// 	if err != nil {
// 		postgresC.Terminate(ctx)
// 		return nil, nil, fmt.Errorf("could not connect to database: %w", err)
// 	}

// 	// Ensure the database is reachable
// 	for i := 0; i < 10; i++ {
// 		err = dbConn.Ping()
// 		if err == nil {
// 			break
// 		}
// 	}

// 	if err != nil {
// 		dbConn.Close()
// 		postgresC.Terminate(ctx)
// 		return nil, nil, fmt.Errorf("database not reachable: %w", err)
// 	}

// 	cleanup := func() {
// 		dbConn.Close()
// 		postgresC.Terminate(ctx)
// 	}

// 	return dbConn, cleanup, nil
// }

// // TestMain sets up the shared database and environment for all tests
// func TestMain(m *testing.M) {
// 	var err error

// 	// Set up the PostgreSQL container once for all tests
// 	sharedDB, cleanupDB, err = setupPostgresContainer()
// 	if err != nil {
// 		fmt.Printf("Could not set up PostgreSQL container: %v\n", err)
// 		os.Exit(1)
// 	}

// 	defer cleanupDB()

// 	// Initialize shared logger and mock event publisher
// 	mockLogger = zerolog.New(os.Stdout).With().Timestamp().Logger()
// 	mockPublisher = &MockEventPublisher{}

// 	// Initialize shared WorkspaceDB instance
// 	workspaceDB = &db.WorkspaceDB{
// 		DB:     sharedDB,
// 		Events: mockPublisher,
// 		Log:    &mockLogger,
// 	}

// 	// Initialize database tables for the tests
// 	err = workspaceDB.InitTables()
// 	if err != nil {
// 		fmt.Printf("Could not initialize tables: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// Run all tests
// 	code := m.Run()
// 	os.Exit(code)
// }
