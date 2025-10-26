package e2e_tests

import (
	"net/http"
	"testing"
	"time"
)

func TestSimpleHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Test if your actual server is running
	resp, err := client.Get("http://localhost:8080/api/health")
	if err != nil {
		t.Skipf("Server not running, skipping E2E tests: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
