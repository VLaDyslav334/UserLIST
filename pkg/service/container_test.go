// actually do not working test yet
package service

import (
	"context"
	"fmt"
	"os"
	"testing"

	"UserLIST/pkg/repository"
	"UserLIST/todo"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestContainers(t *testing.T) (*testContainers, string) {
	err := godotenv.Load("../../.env")
	require.NoError(t, err)

	ctx := context.Background()

	// Configure container request for Windows
	req := testcontainers.ContainerRequest{
		Image:        "postgres:14-alpine",
		ExposedPorts: []string{"5436/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections"),
		Env: map[string]string{
			"POSTGRES_DB":       os.Getenv("POSTGRES_DB"),
			"POSTGRES_USER":     os.Getenv("POSTGRES_USER"),
			"POSTGRES_PASSWORD": os.Getenv("POSTGRES_PASSWORD"),
		},
		Name: "test-postgres",
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Get host and port
	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)
	port, err := pgContainer.MappedPort(ctx, "5436")
	require.NoError(t, err)

	// Build connection string
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host,
		port.Port(),
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_DB"),
	)

	return &testContainers{
		pgContainer: pgContainer,
		ctx:         ctx,
	}, connString
}

// Update testContainers struct
type testContainers struct {
	pgContainer testcontainers.Container
	ctx         context.Context
}

func TestTodoListService_ContainerIntegration(t *testing.T) {
	// Setup containers
	containers, connString := setupTestContainers(t)
	defer func() {
		if err := containers.pgContainer.Terminate(containers.ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// Initialize repository
	db, err := repository.NewPostgresDB(repository.Config{
		Host: connString,
	})
	require.NoError(t, err)
	defer db.Close()

	// Initialize repository and service
	repos := repository.NewRepository(db)
	listService := NewTodoListService(repos.TodoList)

	// Test cases
	t.Run("Full List Lifecycle", func(t *testing.T) {
		userId := 1 // This should match the user created in migrations

		// Create list
		list := todo.TodoList{
			Title:       "Container Test List",
			Description: "Testing with real container config",
		}

		// Create
		listId, err := listService.Create(userId, list)
		require.NoError(t, err)
		require.Greater(t, listId, 0)

		// Get by ID
		retrieved, err := listService.GetById(userId, listId)
		require.NoError(t, err)
		require.Equal(t, list.Title, retrieved.Title)
		require.Equal(t, list.Description, retrieved.Description)

		// Update
		updateInput := todo.UpdateListInput{
			Title:       stringPtr("Updated Container Test List"),
			Description: stringPtr("Updated description"),
		}
		err = listService.Update(userId, listId, updateInput)
		require.NoError(t, err)

		// Get all
		lists, err := listService.GetAll(userId)
		require.NoError(t, err)
		require.NotEmpty(t, lists)
		found := false
		for _, l := range lists {
			if l.Id == listId {
				require.Equal(t, *updateInput.Title, l.Title)
				require.Equal(t, *updateInput.Description, l.Description)
				found = true
				break
			}
		}
		require.True(t, found, "Updated list should be in the results")

		// Delete
		err = listService.Delete(userId, listId)
		require.NoError(t, err)

		// Verify deletion
		_, err = listService.GetById(userId, listId)
		require.Error(t, err)
	})
}
