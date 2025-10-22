package repository

import (
	"fmt"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func TestDatabaseConnection(t *testing.T) {
	getenv := func(key string) string {
		return os.Getenv(key)
	}

	host := getenv("DB_HOST")
	port := getenv("DB_PORT")
	user := getenv("DB_USER")
	password := getenv("DB_PASSWORD")
	dbname := getenv("DB_NAME")
	sslmode := getenv("DB_SSLMODE")

	// Do not run the test unless all required env vars are provided.
	if host == "" || port == "" || user == "" || password == "" || dbname == "" || sslmode == "" {
		t.Skip("Skipping database test; set DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME and DB_SSLMODE environment variables to run")
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	t.Log("Successfully connected to database")
}
