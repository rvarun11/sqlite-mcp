package repository

import (
	"os"
	"testing"

	"github.com/rvarun11/sqlite-mcp/internal/logger"
)

func setupTestDB(t *testing.T) (*SQLiteDB, func()) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpfile.Close()

	logger := logger.NewTestLogger()
	db, err := NewSQLiteDB(tmpfile.Name(), logger)
	if err != nil {
		logger.Errorf("Failed to initialize test database: %v", err)
	}

	// Create test table
	_, err = db.Execute(`
        CREATE TABLE test_users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT UNIQUE,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		db.Close()
		os.Remove(tmpfile.Name())
		t.Fatalf("Failed to create test table: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpfile.Name())
	}

	return db, cleanup
}

func TestGetSchema(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tables, err := db.GetSchema()
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}

	if len(tables) != 1 {
		t.Fatalf("Expected 1 table, got %d", len(tables))
	}

	if tables[0].Name != "test_users" {
		t.Errorf("Expected table name 'test_users', got '%s'", tables[0].Name)
	}

	if len(tables[0].Columns) != 4 {
		t.Errorf("Expected 4 columns, got %d", len(tables[0].Columns))
	}
}

func TestQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert test data
	_, err := db.Execute("INSERT INTO test_users (name, email) VALUES ('John Doe', 'john@example.com')")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Test query
	result, err := db.Query("SELECT * FROM test_users")
	if err != nil {
		t.Fatalf("QueryDatabase failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("Expected 1 row, got %d", result.Count)
	}

	if len(result.Columns) != 4 {
		t.Errorf("Expected 4 columns, got %d", len(result.Columns))
	}
}

func TestExecute(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	result, err := db.Execute("INSERT INTO test_users (name, email) VALUES ('Jane Doe', 'jane@example.com')")
	if err != nil {
		t.Fatalf("ExecuteDatabase failed: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", result.RowsAffected)
	}

	if result.LastInsertId == 0 {
		t.Error("Expected non-zero last insert ID")
	}
}

func TestQueryValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test that INSERT is not allowed in QueryDatabase
	_, err := db.Query("INSERT INTO test_users (name) VALUES ('test')")
	if err == nil {
		t.Error("Expected error for INSERT in QueryDatabase")
	}

	// Test that SELECT is not allowed in ExecuteDatabase
	_, err = db.Execute("SELECT * FROM test_users")
	if err == nil {
		t.Error("Expected error for SELECT in ExecuteDatabase")
	}
}
