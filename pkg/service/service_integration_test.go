package service

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"UserLIST/pkg/repository"
	"UserLIST/todo"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, error) {
	// Connect to default postgres database first
	connInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=%s dbname=%s",
		"localhost", "5432", "postgres", "qwerty", "disable", "postgres")

	db, err := sql.Open("postgres", connInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	// Drop test database if it exists
	_, err = db.Exec("DROP DATABASE IF EXISTS postgres_test")
	if err != nil {
		return nil, fmt.Errorf("failed to drop test database: %v", err)
	}

	// Create test database
	_, err = db.Exec("CREATE DATABASE postgres_test")
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %v", err)
	}

	// Close connection to postgres
	db.Close()

	// Connect to test database
	connInfo = fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=%s dbname=%s",
		"localhost", "5432", "postgres", "qwerty", "disable", "postgres_test")

	testDB, err := sql.Open("postgres", connInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %v", err)
	}

	// Create necessary tables
	_, err = testDB.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id serial primary key,
            name varchar(255) not null,
            username varchar(255) not null unique,
            password_hash varchar(255) not null
        );

        CREATE TABLE IF NOT EXISTS todo_lists (
            id serial primary key,
            title varchar(255) not null,
            description varchar(255)
        );

        CREATE TABLE IF NOT EXISTS users_lists (
            id serial primary key,
            user_id int references users(id) on delete cascade,
            list_id int references todo_lists(id) on delete cascade
        );

        CREATE TABLE IF NOT EXISTS todo_items (
            id serial primary key,
            title varchar(255) not null,
            description varchar(255),
            done boolean not null default false
        );

        CREATE TABLE IF NOT EXISTS lists_items (
            id serial primary key,
            list_id int references todo_lists(id) on delete cascade,
            item_id int references todo_items(id) on delete cascade
        );

        -- Insert test user
        INSERT INTO users (name, username, password_hash) 
        VALUES ('Test User', 'testuser', 'testhash');
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return testDB, nil
}

func TestTodoListService_Integration(t *testing.T) {
	// Setup environment
	setupTestEnv()
	defer cleanupTestEnv()

	// Setup test database
	testDB, err := setupTestDB(t)
	require.NoError(t, err)
	defer testDB.Close()

	// Initialize repository with sqlx wrapper
	db, err := repository.NewPostgresDB(repository.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Username: os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	})
	require.NoError(t, err)
	defer db.Close()
	// Initialize real repositories
	repos := repository.NewRepository(db)

	// Initialize real service
	services := NewTodoListService(repos.TodoList)

	t.Run("Full List Flow", func(t *testing.T) {
		// Create a user first (since we need userId)
		userId := 1 // Assuming user exists

		// Test Create
		list := todo.TodoList{
			Title:       "Integration Test List",
			Description: "Testing full flow",
		}
		listId, err := services.Create(userId, list)
		require.NoError(t, err)
		require.Greater(t, listId, 0)

		// Test GetById
		retrievedList, err := services.GetById(userId, listId)
		require.NoError(t, err)
		require.Equal(t, list.Title, retrievedList.Title)
		require.Equal(t, list.Description, retrievedList.Description)

		// Test Update
		updateInput := todo.UpdateListInput{
			Title:       stringPtr("Updated Title"),
			Description: stringPtr("Updated Description"),
		}
		err = services.Update(userId, listId, updateInput)
		require.NoError(t, err)

		// Verify Update
		updatedList, err := services.GetById(userId, listId)
		require.NoError(t, err)
		require.Equal(t, *updateInput.Title, updatedList.Title)
		require.Equal(t, *updateInput.Description, updatedList.Description)

		// Test Delete
		err = services.Delete(userId, listId)
		require.NoError(t, err)

		// Verify Delete
		_, err = services.GetById(userId, listId)
		require.Error(t, err) // Should return error as list is deleted
	})
}

func stringPtr(s string) *string {
	return &s
}
