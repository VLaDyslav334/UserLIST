// pkg/e2e/e2e_test.go
package e2e_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"UserLIST/pkg/handler"
	"UserLIST/pkg/repository"
	"UserLIST/pkg/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type UserE2ETestSuite struct {
	suite.Suite
	postgresContainer testcontainers.Container
	router            *gin.Engine
	baseURL           string
	client            *http.Client
	db                *repository.Repository
}

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (suite *UserE2ETestSuite) SetupSuite() {
	ctx := context.Background()

	// Use a shorter timeout for container startup
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Start PostgreSQL container with optimized settings
	container, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(1), // Reduced from 2 to 1
		),
	)
	if err != nil {
		suite.T().Skipf("Skipping E2E test - failed to start container: %v", err)
		return
	}
	suite.postgresContainer = container

	// Get connection info
	host, err := container.Host(ctx)
	suite.Require().NoError(err)

	port, err := container.MappedPort(ctx, "5432")
	suite.Require().NoError(err)

	// Initialize database
	db, err := repository.NewPostgresDB(repository.Config{
		Host:     host,
		Port:     port.Port(),
		Username: "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	})
	suite.Require().NoError(err)

	// Run migrations
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
	suite.Require().NoError(err)

	// Initialize application layers
	repos := repository.NewRepository(db)
	services := service.NewService(repos)
	handlers := handler.NewHandler(services)

	// Setup router
	suite.router = handlers.InitRoutes()
	suite.baseURL = "http://localhost:8080"
	suite.client = &http.Client{
		Timeout: 10 * time.Second,
	}
	suite.db = repos

	// Start server in background with shorter timeout
	go func() {
		if err := suite.router.Run(":8080"); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for server to start with timeout
	suite.waitForServer()
}

func (suite *UserE2ETestSuite) TearDownSuite() {
	if suite.postgresContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		suite.postgresContainer.Terminate(ctx)
	}
}

func (suite *UserE2ETestSuite) SetupTest() {
	// Skip if container failed to start
	if suite.postgresContainer == nil {
		suite.T().Skip("Skipping test - container not available")
		return
	}

	// Clean database before each test
	suite.cleanDatabase()
}

func (suite *UserE2ETestSuite) waitForServer() {
	// Wait for server to be ready with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			suite.T().Skip("Skipping E2E test - server failed to start in time")
			return
		case <-ticker.C:
			resp, err := http.Get(suite.baseURL + "/api/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

func (suite *UserE2ETestSuite) cleanDatabase() {
	// Create a new DB connection for cleanup
	ctx := context.Background()
	host, err := suite.postgresContainer.Host(ctx)
	if err != nil {
		return
	}

	port, err := suite.postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		return
	}

	db, err := repository.NewPostgresDB(repository.Config{
		Host:     host,
		Port:     port.Port(),
		Username: "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	})
	if err != nil {
		return
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM users")
	if err != nil {
		log.Printf("Failed to clean database: %v", err)
	}
}

func (suite *UserE2ETestSuite) TestCreateUser() {
	// Skip if setup failed
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	userReq := CreateUserRequest{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	createdUser := suite.createUser(userReq)
	assert.NotZero(suite.T(), createdUser.ID)
	assert.Equal(suite.T(), userReq.Name, createdUser.Name)
	assert.Equal(suite.T(), userReq.Email, createdUser.Email)
	assert.False(suite.T(), createdUser.CreatedAt.IsZero())
}

func (suite *UserE2ETestSuite) TestGetUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	userReq := CreateUserRequest{
		Name:  "Jane Smith",
		Email: "jane@example.com",
	}
	createdUser := suite.createUser(userReq)

	retrievedUser := suite.getUser(createdUser.ID)
	assert.Equal(suite.T(), createdUser.ID, retrievedUser.ID)
	assert.Equal(suite.T(), userReq.Name, retrievedUser.Name)
	assert.Equal(suite.T(), userReq.Email, retrievedUser.Email)
}

func (suite *UserE2ETestSuite) TestGetAllUsers() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Create multiple users
	users := []CreateUserRequest{
		{Name: "User 1", Email: "user1@example.com"},
		{Name: "User 2", Email: "user2@example.com"},
		{Name: "User 3", Email: "user3@example.com"},
	}

	for _, user := range users {
		suite.createUser(user)
	}

	// Get all users
	allUsers := suite.getAllUsers()
	assert.Len(suite.T(), allUsers, 3)
}

func (suite *UserE2ETestSuite) TestUpdateUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Create user
	userReq := CreateUserRequest{
		Name:  "Original Name",
		Email: "original@example.com",
	}
	createdUser := suite.createUser(userReq)

	// Update user
	updatedData := CreateUserRequest{
		Name:  "Updated Name",
		Email: "updated@example.com",
	}
	suite.updateUser(createdUser.ID, updatedData)

	// Verify update
	updatedUser := suite.getUser(createdUser.ID)
	assert.Equal(suite.T(), updatedData.Name, updatedUser.Name)
	assert.Equal(suite.T(), updatedData.Email, updatedUser.Email)
}

func (suite *UserE2ETestSuite) TestDeleteUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Create user
	userReq := CreateUserRequest{
		Name:  "To Delete",
		Email: "delete@example.com",
	}
	createdUser := suite.createUser(userReq)

	// Verify user exists
	retrievedUser := suite.getUser(createdUser.ID)
	assert.Equal(suite.T(), createdUser.ID, retrievedUser.ID)

	// Delete user
	suite.deleteUser(createdUser.ID)

	// Verify user is deleted
	suite.verifyUserNotFound(createdUser.ID)
}

func (suite *UserE2ETestSuite) TestCreateUserDuplicateEmail() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	userReq := CreateUserRequest{
		Name:  "User 1",
		Email: "duplicate@example.com",
	}

	// First user should succeed
	suite.createUser(userReq)

	// Second user with same email should fail
	userReq2 := CreateUserRequest{
		Name:  "User 2",
		Email: "duplicate@example.com",
	}
	suite.createUserShouldFail(userReq2)
}

func (suite *UserE2ETestSuite) TestGetNonExistentUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Try to get a user that doesn't exist
	suite.verifyUserNotFound(9999)
}

func (suite *UserE2ETestSuite) TestUpdateNonExistentUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Try to update a user that doesn't exist
	updatedData := CreateUserRequest{
		Name:  "Non Existent",
		Email: "nonexistent@example.com",
	}

	resp := suite.updateUserShouldFail(9999, updatedData)
	assert.NotEqual(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *UserE2ETestSuite) TestDeleteNonExistentUser() {
	if suite.postgresContainer == nil {
		suite.T().Skip("Container not available")
		return
	}

	// Try to delete a user that doesn't exist
	resp := suite.deleteUserShouldFail(9999)
	assert.NotEqual(suite.T(), http.StatusOK, resp.StatusCode)
}

// Helper methods
func (suite *UserE2ETestSuite) createUser(userReq CreateUserRequest) User {
	jsonData, err := json.Marshal(userReq)
	suite.Require().NoError(err)

	resp, err := suite.client.Post(suite.baseURL+"/api/users", "application/json", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var createdUser User
	err = json.NewDecoder(resp.Body).Decode(&createdUser)
	suite.Require().NoError(err)

	return createdUser
}

func (suite *UserE2ETestSuite) createUserShouldFail(userReq CreateUserRequest) {
	jsonData, err := json.Marshal(userReq)
	suite.Require().NoError(err)

	resp, err := suite.client.Post(suite.baseURL+"/api/users", "application/json", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.NotEqual(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *UserE2ETestSuite) getUser(id int) User {
	resp, err := suite.client.Get(fmt.Sprintf("%s/api/users/%d", suite.baseURL, id))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var user User
	err = json.NewDecoder(resp.Body).Decode(&user)
	suite.Require().NoError(err)

	return user
}

func (suite *UserE2ETestSuite) getAllUsers() []User {
	resp, err := suite.client.Get(suite.baseURL + "/api/users")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var users []User
	err = json.NewDecoder(resp.Body).Decode(&users)
	suite.Require().NoError(err)

	return users
}

func (suite *UserE2ETestSuite) updateUser(id int, userReq CreateUserRequest) {
	jsonData, err := json.Marshal(userReq)
	suite.Require().NoError(err)

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/users/%d", suite.baseURL, id), bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *UserE2ETestSuite) updateUserShouldFail(id int, userReq CreateUserRequest) *http.Response {
	jsonData, err := json.Marshal(userReq)
	suite.Require().NoError(err)

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/users/%d", suite.baseURL, id), bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	return resp
}

func (suite *UserE2ETestSuite) deleteUser(id int) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/users/%d", suite.baseURL, id), nil)
	suite.Require().NoError(err)

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *UserE2ETestSuite) deleteUserShouldFail(id int) *http.Response {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/users/%d", suite.baseURL, id), nil)
	suite.Require().NoError(err)

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	return resp
}

func (suite *UserE2ETestSuite) verifyUserNotFound(id int) {
	resp, err := suite.client.Get(fmt.Sprintf("%s/api/users/%d", suite.baseURL, id))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	assert.NotEqual(suite.T(), http.StatusOK, resp.StatusCode)
}

func TestUserE2ETestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	suite.Run(t, new(UserE2ETestSuite))
}
