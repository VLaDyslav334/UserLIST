package service

import "os"

func setupTestEnv() {
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "qwerty")
	os.Setenv("DB_NAME", "postgres_test")
	os.Setenv("DB_SSLMODE", "disable")
}

func cleanupTestEnv() {
	vars := []string{
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSLMODE",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}
