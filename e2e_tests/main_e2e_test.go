package e2e_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	timeout = 30 * time.Second
)

var (
	baseURL       string
	postgresC     testcontainers.Container
	appC          testcontainers.Container
	testCtx       context.Context
	testCtxCancel context.CancelFunc
)

// Test data structures matching your API
type SignUpRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type CreateListRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ListResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type CreateItemRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

type ItemResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

type UpdateItemRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Done        *bool   `json:"done,omitempty"`
}

type UpdateListRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Helper function to create HTTP client with timeout
func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// Helper function to make HTTP requests
func makeRequest(t *testing.T, method, url string, body interface{}, token string) (*http.Response, []byte) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err, "Failed to marshal request body")
		reqBody = bytes.NewBuffer(jsonData)
		t.Logf("Request body: %s", string(jsonData))
	}

	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	t.Logf("Making %s request to %s", method, url)
	client := getHTTPClient()
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	t.Logf("Response status: %d, body: %s", resp.StatusCode, string(respBody))
	return resp, respBody
}

// Helper function to create a test user and return token
func createTestUser(t *testing.T) (string, SignUpRequest) {
	t.Helper()

	timestamp := time.Now().UnixNano()
	userData := SignUpRequest{
		Name:     fmt.Sprintf("Test User %d", timestamp),
		Username: fmt.Sprintf("testuser_%d", timestamp),
		Password: "SecurePass123!",
	}

	resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-up", userData, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to create test user")

	var authResp AuthResponse
	err := json.Unmarshal(body, &authResp)
	require.NoError(t, err, "Failed to unmarshal auth response")
	require.NotEmpty(t, authResp.Token, "Token should not be empty")

	t.Logf("Created test user: %s with token", userData.Username)
	return authResp.Token, userData
}

// setupTestContainers initializes PostgreSQL and application containers
func setupTestContainers() error {
	var err error
	testCtx, testCtxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	// Create network for containers to communicate
	networkName := "test-network"
	network, err := testcontainers.GenericNetwork(testCtx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	defer func() {
		if err != nil && network != nil {
			_ = network.Remove(testCtx)
		}
	}()

	fmt.Println("Setting up PostgreSQL container...")

	// PostgreSQL container
	postgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:17-bookworm",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"postgres"},
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(2 * time.Minute),
	}

	postgresC, err = testcontainers.GenericContainer(testCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: postgresReq,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get mapped port for PostgreSQL
	postgresPort, err := postgresC.MappedPort(testCtx, "5432")
	if err != nil {
		return fmt.Errorf("failed to get postgres port: %w", err)
	}

	fmt.Printf("PostgreSQL container started on port %s\n", postgresPort.Port())

	// Wait a bit for PostgreSQL to fully initialize
	time.Sleep(3 * time.Second)

	fmt.Println("Building and starting application container...")

	// Application container
	appReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../", // Path to your project root with Dockerfile
			Dockerfile:    "Dockerfile",
			PrintBuildLog: true,
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"DB_HOST":     "postgres",
			"DB_PORT":     "5432",
			"DB_USER":     "testuser",
			"DB_PASSWORD": "testpass",
			"DB_NAME":     "testdb",
			"DB_SSLMODE":  "disable",
			"JWT_SECRET":  "test-secret-key",
		},
		Networks: []string{networkName},
		WaitingFor: wait.ForAll(
			wait.ForHTTP("/api/health").WithPort("8080/tcp").WithStatusCodeMatcher(
				func(status int) bool {
					return status == http.StatusOK
				},
			),
			wait.ForLog("Server started"),
		).WithDeadline(3 * time.Minute),
	}

	appC, err = testcontainers.GenericContainer(testCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: appReq,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start app container: %w", err)
	}

	// Get mapped port for application
	appPort, err := appC.MappedPort(testCtx, "8080")
	if err != nil {
		return fmt.Errorf("failed to get app port: %w", err)
	}

	baseURL = fmt.Sprintf("http://localhost:%s", appPort.Port())
	fmt.Printf("Application container started on %s\n", baseURL)

	// Verify application is ready
	fmt.Println("Verifying application health...")
	client := getHTTPClient()
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Application is healthy and ready for tests!")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("  Waiting for app to be ready... (%d/%d)\n", i+1, maxRetries)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("application did not become healthy in time")
}

// teardownTestContainers cleans up all containers
func teardownTestContainers() {
	fmt.Println("\n Cleaning up test containers...")

	if appC != nil {
		if err := appC.Terminate(testCtx); err != nil {
			fmt.Printf("Failed to terminate app container: %v\n", err)
		} else {
			fmt.Println("Application container terminated")
		}
	}

	if postgresC != nil {
		if err := postgresC.Terminate(testCtx); err != nil {
			fmt.Printf("Failed to terminate postgres container: %v\n", err)
		} else {
			fmt.Println("PostgreSQL container terminated")
		}
	}

	if testCtxCancel != nil {
		testCtxCancel()
	}

	fmt.Println("Cleanup complete")
}

// TestHealthCheck verifies that the API is running
func TestHealthCheck(t *testing.T) {
	resp, body := makeRequest(t, "GET", baseURL+"/api/health", nil, "")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	var healthResp map[string]interface{}
	err := json.Unmarshal(body, &healthResp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", healthResp["status"])

	t.Logf("Health check passed: %v", healthResp)
}

// TestDebugRoutes helps verify all routes are registered
func TestDebugRoutes(t *testing.T) {
	resp, body := makeRequest(t, "GET", baseURL+"/api/debug/routes", nil, "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Logf("Available routes: %s", string(body))
}

// TestSignUpSuccess tests successful user registration
func TestSignUpSuccess(t *testing.T) {
	timestamp := time.Now().UnixNano()
	signUpData := SignUpRequest{
		Name:     fmt.Sprintf("John Doe %d", timestamp),
		Username: fmt.Sprintf("johndoe_%d", timestamp),
		Password: "SecurePass123!",
	}

	resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-up", signUpData, "")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Sign up should return 200")

	var authResp AuthResponse
	err := json.Unmarshal(body, &authResp)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.NotEmpty(t, authResp.Token, "Token should not be empty")
	t.Logf("User created successfully with token: %s...", authResp.Token[:min(20, len(authResp.Token))])
}

// TestSignUpDuplicateUsername tests registration with existing username
func TestSignUpDuplicateUsername(t *testing.T) {
	timestamp := time.Now().UnixNano()
	username := fmt.Sprintf("duplicate_%d", timestamp)

	signUpData := SignUpRequest{
		Name:     "User One",
		Username: username,
		Password: "SecurePass123!",
	}

	// First registration - should succeed
	resp, _ := makeRequest(t, "POST", baseURL+"/auth/sign-up", signUpData, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "First registration should succeed")

	// Second registration with same username - should fail
	signUpData.Name = "User Two"
	resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-up", signUpData, "")

	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Duplicate username should not return 200")
	t.Logf("Duplicate registration response: %s", string(body))
}

// TestSignUpInvalidData tests registration with invalid data
func TestSignUpInvalidData(t *testing.T) {
	tests := []struct {
		name     string
		data     SignUpRequest
		wantFail bool
	}{
		{
			name:     "Empty username",
			data:     SignUpRequest{Name: "Test", Username: "", Password: "Pass123!"},
			wantFail: true,
		},
		{
			name:     "Empty password",
			data:     SignUpRequest{Name: "Test", Username: "testuser", Password: ""},
			wantFail: true,
		},
		{
			name:     "Empty name",
			data:     SignUpRequest{Name: "", Username: "testuser", Password: "Pass123!"},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-up", tt.data, "")
			if tt.wantFail {
				assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Should fail with invalid data")
				t.Logf("Expected failure response: %s", string(body))
			}
		})
	}
}

// TestSignInSuccess tests successful login
func TestSignInSuccess(t *testing.T) {
	// Create a user first
	timestamp := time.Now().UnixNano()
	username := fmt.Sprintf("signinuser_%d", timestamp)
	password := "SecurePass123!"

	signUpData := SignUpRequest{
		Name:     "Sign In Test User",
		Username: username,
		Password: password,
	}

	resp, _ := makeRequest(t, "POST", baseURL+"/auth/sign-up", signUpData, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "User creation failed")

	// Now sign in
	signInData := SignInRequest{
		Username: username,
		Password: password,
	}

	resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-in", signInData, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Sign in should return 200")

	var authResp AuthResponse
	err := json.Unmarshal(body, &authResp)
	require.NoError(t, err)
	assert.NotEmpty(t, authResp.Token, "Token should not be empty")

	t.Logf("Sign in successful with token: %s...", authResp.Token[:min(20, len(authResp.Token))])
}

// TestSignInInvalidCredentials tests login with wrong credentials
func TestSignInInvalidCredentials(t *testing.T) {
	signInData := SignInRequest{
		Username: "nonexistent_user",
		Password: "WrongPassword123!",
	}

	resp, body := makeRequest(t, "POST", baseURL+"/auth/sign-in", signInData, "")
	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Invalid credentials should not return 200")

	t.Logf("Expected error response: %s", string(body))
}

// TestListLifecycle tests complete CRUD operations for todo lists
func TestListLifecycle(t *testing.T) {
	// 1. Create a user and get token
	token, _ := createTestUser(t)

	// 2. Create a list
	listData := CreateListRequest{
		Title:       "My Test List",
		Description: "This is a test todo list",
	}

	resp, body := makeRequest(t, "POST", baseURL+"/api/lists/", listData, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "List creation should return 200")

	var createdList map[string]interface{}
	err := json.Unmarshal(body, &createdList)
	require.NoError(t, err)

	// Get the list ID
	var listID int
	if id, ok := createdList["id"].(float64); ok {
		listID = int(id)
	}
	require.NotZero(t, listID, "List ID should not be zero")
	t.Logf("Created list with ID: %d", listID)

	// 3. Get all lists
	resp, body = makeRequest(t, "GET", baseURL+"/api/lists/", nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Get all lists should return 200")

	var listsResp map[string]interface{}
	err = json.Unmarshal(body, &listsResp)
	require.NoError(t, err)
	t.Logf("All lists response: %v", listsResp)

	// 4. Get specific list
	resp, body = makeRequest(t, "GET", fmt.Sprintf("%s/api/lists/%d", baseURL, listID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Get list by ID should return 200")
	t.Logf("Get list response: %s", string(body))

	// 5. Update list
	updatedTitle := "Updated Test List"
	updateData := UpdateListRequest{
		Title: &updatedTitle,
	}

	resp, body = makeRequest(t, "PUT", fmt.Sprintf("%s/api/lists/%d", baseURL, listID), updateData, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Update list should return 200")
	t.Logf("Update list response: %s", string(body))

	// 6. Delete list
	resp, _ = makeRequest(t, "DELETE", fmt.Sprintf("%s/api/lists/%d", baseURL, listID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Delete list should return 200")
	t.Logf("List deleted successfully")

	// 7. Verify deletion
	resp, _ = makeRequest(t, "GET", fmt.Sprintf("%s/api/lists/%d", baseURL, listID), nil, token)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Getting deleted list should not return 200")
}

// TestItemLifecycle tests CRUD operations for list items
func TestItemLifecycle(t *testing.T) {
	// 1. Setup: Create user and list
	token, _ := createTestUser(t)

	listData := CreateListRequest{
		Title:       "List for Items",
		Description: "Testing items in this list",
	}

	resp, body := makeRequest(t, "POST", baseURL+"/api/lists/", listData, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var createdList map[string]interface{}
	json.Unmarshal(body, &createdList)

	var listID int
	if id, ok := createdList["id"].(float64); ok {
		listID = int(id)
	}
	require.NotZero(t, listID)
	t.Logf("Created list with ID: %d for items", listID)

	// 2. Create an item in the list
	itemData := CreateItemRequest{
		Title:       "Test Item",
		Description: "This is a test item",
		Done:        false,
	}

	resp, body = makeRequest(t, "POST", fmt.Sprintf("%s/api/lists/%d/items/", baseURL, listID), itemData, token)
	require.Equal(t, http.StatusOK, resp.StatusCode, "Item creation should return 200")

	var createdItem map[string]interface{}
	err := json.Unmarshal(body, &createdItem)
	require.NoError(t, err)

	var itemID int
	if id, ok := createdItem["id"].(float64); ok {
		itemID = int(id)
	}
	require.NotZero(t, itemID, "Item ID should not be zero")
	t.Logf("Created item with ID: %d", itemID)

	// 3. Get all items in the list
	resp, body = makeRequest(t, "GET", fmt.Sprintf("%s/api/lists/%d/items/", baseURL, listID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Get all items should return 200")
	t.Logf("All items response: %s", string(body))

	// 4. Get specific item
	resp, body = makeRequest(t, "GET", fmt.Sprintf("%s/api/items/%d", baseURL, itemID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Get item by ID should return 200")
	t.Logf("Get item response: %s", string(body))

	// 5. Update item
	done := true
	updatedTitle := "Updated Item"
	updateData := UpdateItemRequest{
		Title: &updatedTitle,
		Done:  &done,
	}

	resp, body = makeRequest(t, "PUT", fmt.Sprintf("%s/api/items/%d", baseURL, itemID), updateData, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Update item should return 200")
	t.Logf("Update item response: %s", string(body))

	// 6. Delete item
	resp, _ = makeRequest(t, "DELETE", fmt.Sprintf("%s/api/items/%d", baseURL, itemID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Delete item should return 200")
	t.Logf("Item deleted successfully")

	// 7. Verify deletion
	resp, _ = makeRequest(t, "GET", fmt.Sprintf("%s/api/items/%d", baseURL, itemID), nil, token)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Getting deleted item should not return 200")
}

// TestUnauthorizedAccess tests accessing protected routes without token
func TestUnauthorizedAccess(t *testing.T) {
	tests := []struct {
		name   string
		method string
		url    string
	}{
		{"Get lists", "GET", baseURL + "/api/lists/"},
		{"Create list", "POST", baseURL + "/api/lists/"},
		{"Get list by ID", "GET", baseURL + "/api/lists/1"},
		{"Update list", "PUT", baseURL + "/api/lists/1"},
		{"Delete list", "DELETE", baseURL + "/api/lists/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(t, tt.method, tt.url, nil, "")
			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				"Request without token should not return 200")
			t.Logf("Unauthorized response: %d - %s", resp.StatusCode, string(body))
		})
	}
}

// TestCompleteWorkflow tests a realistic user workflow
func TestCompleteWorkflow(t *testing.T) {
	// 1. User signs up
	token, userData := createTestUser(t)
	t.Logf("Step 1: User '%s' signed up", userData.Username)

	// 2. User creates a shopping list
	listData := CreateListRequest{
		Title:       "Shopping List",
		Description: "Weekly groceries",
	}
	resp, body := makeRequest(t, "POST", baseURL+"/api/lists/", listData, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var createdList map[string]interface{}
	json.Unmarshal(body, &createdList)
	var listID int
	if id, ok := createdList["id"].(float64); ok {
		listID = int(id)
	}
	require.NotZero(t, listID)
	t.Logf("Step 2: Created shopping list with ID: %d", listID)

	// 3. Add items to the list
	items := []string{"Milk", "Bread", "Eggs"}
	for _, itemTitle := range items {
		itemData := CreateItemRequest{
			Title:       itemTitle,
			Description: fmt.Sprintf("Buy %s", itemTitle),
			Done:        false,
		}
		resp, _ := makeRequest(t, "POST", fmt.Sprintf("%s/api/lists/%d/items/", baseURL, listID), itemData, token)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to create item: "+itemTitle)
	}
	t.Logf("Step 3: Added %d items to the list", len(items))

	// 4. View all lists
	resp, _ = makeRequest(t, "GET", baseURL+"/api/lists/", nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Log("Step 4: Viewed all lists")

	// 5. View items in the list
	resp, body = makeRequest(t, "GET", fmt.Sprintf("%s/api/lists/%d/items/", baseURL, listID), nil, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Logf("Step 5: Viewed items in the list")

	t.Log("Complete workflow test passed!")
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestMain runs before all tests to set up and tear down containers
func TestMain(m *testing.M) {
	// Setup
	fmt.Println("Starting E2E test suite with Testcontainers...")
	if err := setupTestContainers(); err != nil {
		fmt.Printf("Failed to setup test containers: %v\n", err)
		teardownTestContainers()
		os.Exit(1)
	}

	// Run tests
	fmt.Printf("\n Running tests...\n")
	code := m.Run()

	// Teardown
	teardownTestContainers()

	os.Exit(code)
}
