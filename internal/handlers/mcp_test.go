package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rvarun11/sqlite-mcp/internal/logger"
	"github.com/rvarun11/sqlite-mcp/internal/repository"
)

// textFromResult is a helper that type-asserts the first content item to
// mcp.TextContent (value type, as returned by mcp.NewToolResultText /
// mcp.NewToolResultError) and returns its text.
func textFromResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("Expected at least one content item, got 0")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Expected mcp.TextContent (value), got %T", result.Content[0])
	}
	return tc.Text
}

func setupTestMCPHandler(t *testing.T) (*MCPHandler, func()) {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "test_mcp_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpfile.Close()

	log := logger.NewTestLogger()
	repo, err := repository.NewSQLiteDBFromPath(tmpfile.Name(), log)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	_, err = repo.Execute(`
        CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            email TEXT UNIQUE NOT NULL,
            age INTEGER,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		repo.Close()
		os.Remove(tmpfile.Name())
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = repo.Execute(`
        CREATE TABLE orders (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            product_name TEXT NOT NULL,
            quantity INTEGER DEFAULT 1,
            price DECIMAL(10,2),
            FOREIGN KEY (user_id) REFERENCES users(id)
        )
    `)
	if err != nil {
		repo.Close()
		os.Remove(tmpfile.Name())
		t.Fatalf("Failed to create orders table: %v", err)
	}

	_, err = repo.Execute(`CREATE INDEX idx_users_email ON users(email)`)
	if err != nil {
		repo.Close()
		os.Remove(tmpfile.Name())
		t.Fatalf("Failed to create index: %v", err)
	}

	_, err = repo.Execute(`
        INSERT INTO users (name, email, age) VALUES 
        ('John Doe', 'john@example.com', 30),
        ('Jane Smith', 'jane@example.com', 25),
        ('Bob Wilson', 'bob@example.com', 35)
    `)
	if err != nil {
		repo.Close()
		os.Remove(tmpfile.Name())
		t.Fatalf("Failed to insert test data: %v", err)
	}

	handler := NewMCPHandler(repo, log)

	cleanup := func() {
		repo.Close()
		os.Remove(tmpfile.Name())
	}

	return handler, cleanup
}

// setupHandlerNoDB returns an MCPHandler with no database open yet.
func setupHandlerNoDB(t *testing.T) (*MCPHandler, func()) {
	t.Helper()
	log := logger.NewTestLogger()
	repo := repository.NewSQLiteDB(log)
	handler := NewMCPHandler(repo, log)
	cleanup := func() { repo.Close() }
	return handler, cleanup
}

func TestMCPHandler_GetSchema(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.GetSchema(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "get_schema"},
	})
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result, got error")
	}
	if len(result.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Content))
	}

	response := textFromResult(t, result)
	if response == "" {
		t.Error("Expected non-empty response")
	}
	if !containsString(response, "users") {
		t.Error("Expected response to contain 'users' table")
	}
	if !containsString(response, "orders") {
		t.Error("Expected response to contain 'orders' table")
	}
	if !containsString(response, "email") {
		t.Error("Expected response to contain column information")
	}
	if !containsString(response, "Foreign Keys") {
		t.Error("Expected response to contain foreign key information")
	}
}

func TestMCPHandler_Query_Success(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{"sql": "SELECT name, email FROM users WHERE age > 25 ORDER BY name"},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result, got error")
	}
	if len(result.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Content))
	}

	response := textFromResult(t, result)
	if !containsString(response, "Query Results") {
		t.Error("Expected response to contain 'Query Results'")
	}
	if !containsString(response, "Row Count") {
		t.Error("Expected response to contain row count")
	}
	if !containsString(response, "john@example.com") || !containsString(response, "bob@example.com") {
		t.Error("Expected response to contain filtered user data")
	}
}

func TestMCPHandler_Query_MissingSQL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for missing SQL parameter")
	}
	if !containsString(textFromResult(t, result), "SQL query parameter is required") {
		t.Error("Expected error message about missing SQL parameter")
	}
}

func TestMCPHandler_Query_EmptySQL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{"sql": ""},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for empty SQL")
	}
}

func TestMCPHandler_Query_InvalidSQL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{"sql": "SELECT * FROM nonexistent_table"},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for invalid SQL")
	}
	if !containsString(textFromResult(t, result), "Query execution failed") {
		t.Error("Expected error message about query execution failure")
	}
}

func TestMCPHandler_Execute_Success(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"sql": "INSERT INTO users (name, email, age) VALUES ('Test User', 'test@example.com', 28)"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result, got error")
	}
	if len(result.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Content))
	}

	response := textFromResult(t, result)
	if !containsString(response, "Execution Result") {
		t.Error("Expected response to contain 'Execution Result'")
	}
	if !containsString(response, "Rows Affected: 1") {
		t.Error("Expected response to show 1 row affected")
	}
	if !containsString(response, "Last Insert ID") {
		t.Error("Expected response to contain last insert ID")
	}
}

func TestMCPHandler_Execute_Update(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"sql": "UPDATE users SET age = 31 WHERE name = 'John Doe'"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result, got error")
	}
	if !containsString(textFromResult(t, result), "Rows Affected: 1") {
		t.Error("Expected response to show 1 row affected")
	}
}

func TestMCPHandler_Execute_MissingSQL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for missing SQL parameter")
	}
	if !containsString(textFromResult(t, result), "Missing or invalid 'sql' argument") {
		t.Error("Expected error message about missing SQL argument")
	}
}

func TestMCPHandler_Execute_InvalidSQL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"sql": "INSERT INTO nonexistent_table (name) VALUES ('test')"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for invalid SQL")
	}
	if !containsString(textFromResult(t, result), "Statement execution failed") {
		t.Error("Expected error message about statement execution failure")
	}
}

func TestMCPHandler_Execute_DDL(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "execute",
			Arguments: map[string]any{
				"sql": `CREATE TABLE test_table (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL
                )`,
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result for CREATE TABLE")
	}
	if !containsString(textFromResult(t, result), "Statement executed successfully") {
		t.Error("Expected success message for DDL operation")
	}
}

func TestMCPHandler_Query_WithClause(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "query",
			Arguments: map[string]any{
				"sql": `WITH adult_users AS (
                    SELECT name, email FROM users WHERE age >= 30
                ) SELECT * FROM adult_users ORDER BY name`,
			},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result for WITH clause query")
	}
	if !containsString(textFromResult(t, result), "Query Results") {
		t.Error("Expected query results for WITH clause")
	}
}

func TestMCPHandler_Query_Explain(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{"sql": "EXPLAIN SELECT * FROM users WHERE age > 25"},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result.IsError {
		t.Error("Expected successful result for EXPLAIN query")
	}
	if !containsString(textFromResult(t, result), "Query Results") {
		t.Error("Expected query results for EXPLAIN")
	}
}

// --- open_database tests ---

func TestMCPHandler_OpenDatabase_NoDB_GetSchema(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.GetSchema(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when no database is open")
	}
	if !containsString(textFromResult(t, result), "no database is open") {
		t.Errorf("Expected 'no database is open' message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_NoDB_Query(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"sql": "SELECT 1"},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when no database is open")
	}
	if !containsString(textFromResult(t, result), "no database is open") {
		t.Errorf("Expected 'no database is open' message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_NoDB_Execute(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.Execute(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"sql": "CREATE TABLE foo (id INTEGER)"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error when no database is open")
	}
	if !containsString(textFromResult(t, result), "no database is open") {
		t.Errorf("Expected 'no database is open' message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_Success(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	tmpfile, err := os.CreateTemp("", "open_db_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	ctx := context.Background()
	result, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"path": tmpfile.Name()},
		},
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", textFromResult(t, result))
	}
	if !containsString(textFromResult(t, result), "Opened database at") {
		t.Errorf("Expected confirmation message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_NonExistentFile(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"path": "/nonexistent/path/db.sqlite"},
		},
	})
	if err != nil {
		t.Fatalf("OpenDatabase returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for non-existent database file")
	}
	if !containsString(textFromResult(t, result), "Failed to open database") {
		t.Errorf("Expected failure message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_MissingPath(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	ctx := context.Background()
	result, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("OpenDatabase returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for missing path parameter")
	}
	if !containsString(textFromResult(t, result), "Missing or invalid 'path' argument") {
		t.Errorf("Expected missing path message, got: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_SwapsDB(t *testing.T) {
	// Open handler pointing at DB A (has users table), then switch to DB B (empty).
	handler, cleanupA := setupTestMCPHandler(t)
	defer cleanupA()

	tmpB, err := os.CreateTemp("", "open_db_b_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file for DB B: %v", err)
	}
	tmpB.Close()
	defer os.Remove(tmpB.Name())

	ctx := context.Background()

	// Verify DB A has the users table
	result, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"sql": "SELECT count(*) FROM users"},
		},
	})
	if err != nil || result.IsError {
		t.Fatal("Expected query on DB A to succeed")
	}

	// Switch to DB B
	openResult, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"path": tmpB.Name()},
		},
	})
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	if openResult.IsError {
		t.Fatalf("Expected open to succeed, got: %s", textFromResult(t, openResult))
	}

	// DB B has no tables — querying users should now fail
	result2, err := handler.Query(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"sql": "SELECT count(*) FROM users"},
		},
	})
	if err != nil {
		t.Fatalf("Query returned unexpected Go error: %v", err)
	}
	if !result2.IsError {
		t.Error("Expected query to fail after switching to empty database")
	}
}

// --- Concurrency stress test ---

// TestMCPHandler_ConcurrentOpenDatabaseAndQueries hammers the gate RWMutex by
// running many concurrent Query/Execute/GetSchema calls while repeatedly calling
// OpenDatabase to swap the underlying connection. The test asserts:
//
//  1. No goroutine panics (the race detector and recover() catch this).
//  2. Every call returns a non-nil *mcp.CallToolResult and never a Go error.
//  3. After all goroutines finish the handler is still usable.
//
// Run with -race to exercise the race detector:
//
//	go test ./internal/handlers/ -race -run TestMCPHandler_ConcurrentOpenDatabaseAndQueries
func TestMCPHandler_ConcurrentOpenDatabaseAndQueries(t *testing.T) {
	const (
		dataWorkers  = 20 // concurrent query/execute/schema goroutines
		openWorkers  = 5  // concurrent open_database goroutines
		roundsPerGor = 30 // iterations each goroutine performs
	)

	// Create two real SQLite files to alternate between.
	makeDB := func(seed string) string {
		f, err := os.CreateTemp("", "stress_*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		f.Close()
		// Pre-populate so queries have something to work with.
		repo, err := repository.NewSQLiteDBFromPath(f.Name(), logger.NewTestLogger())
		if err != nil {
			t.Fatalf("NewSQLiteDBFromPath: %v", err)
		}
		_, _ = repo.Execute(`CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY, v TEXT)`)
		_, _ = repo.Execute(`INSERT INTO kv VALUES ('` + seed + `','1')`)
		repo.Close()
		return f.Name()
	}

	pathA := makeDB("a")
	pathB := makeDB("b")
	defer os.Remove(pathA)
	defer os.Remove(pathB)

	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	// Seed the handler with pathA so data workers have something to query from
	// the start.
	ctx := context.Background()
	if r, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": pathA}},
	}); err != nil || r.IsError {
		t.Fatalf("initial OpenDatabase failed: err=%v isError=%v", err, r.IsError)
	}

	// failures collects error strings from all goroutines without any risk of
	// blocking: a mutex-protected slice has no capacity limit, so goroutines
	// never stall waiting for a drain that only happens after wg.Wait().
	var (
		wg       sync.WaitGroup
		failMu   sync.Mutex
		failures []string
	)

	recordFailure := func(msg string) {
		failMu.Lock()
		failures = append(failures, msg)
		failMu.Unlock()
	}

	safeCall := func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				recordFailure(fmt.Sprintf("panic: %v", r))
			}
		}()
		fn()
	}

	// Data workers: rotate through Query, Execute, GetSchema.
	for i := range dataWorkers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range roundsPerGor {
				switch id % 3 {
				case 0:
					safeCall(func() {
						r, err := handler.Query(ctx, mcp.CallToolRequest{
							Params: mcp.CallToolParams{
								Arguments: map[string]any{"sql": "SELECT * FROM kv"},
							},
						})
						if err != nil {
							recordFailure(fmt.Sprintf("Query returned Go error: %v", err))
						}
						if r == nil {
							recordFailure("Query returned nil result")
						}
					})
				case 1:
					safeCall(func() {
						r, err := handler.Execute(ctx, mcp.CallToolRequest{
							Params: mcp.CallToolParams{
								Arguments: map[string]any{
									"sql": fmt.Sprintf("INSERT OR REPLACE INTO kv VALUES ('w%d','x')", id),
								},
							},
						})
						if err != nil {
							recordFailure(fmt.Sprintf("Execute returned Go error: %v", err))
						}
						if r == nil {
							recordFailure("Execute returned nil result")
						}
					})
				case 2:
					safeCall(func() {
						r, err := handler.GetSchema(ctx, mcp.CallToolRequest{})
						if err != nil {
							recordFailure(fmt.Sprintf("GetSchema returned Go error: %v", err))
						}
						if r == nil {
							recordFailure("GetSchema returned nil result")
						}
					})
				}
			}
		}(i)
	}

	// Open workers: alternate between pathA and pathB.
	for i := range openWorkers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			paths := [2]string{pathA, pathB}
			for j := range roundsPerGor {
				path := paths[(id+j)%2]
				safeCall(func() {
					r, err := handler.OpenDatabase(ctx, mcp.CallToolRequest{
						Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}},
					})
					if err != nil {
						recordFailure(fmt.Sprintf("OpenDatabase returned Go error: %v", err))
					}
					if r == nil {
						recordFailure("OpenDatabase returned nil result")
					}
				})
			}
		}(i)
	}

	wg.Wait()

	for _, msg := range failures {
		t.Errorf("goroutine failure: %s", msg)
	}

	// Handler must still be functional after all the chaos.
	if r, err := handler.GetSchema(ctx, mcp.CallToolRequest{}); err != nil || r == nil {
		t.Errorf("handler unusable after stress test: err=%v result=%v", err, r)
	}
}

// --- Finding 2: malformed Arguments must not panic ---
// These tests pass nil or a non-map Arguments value to every handler and
// assert that a structured error is returned rather than a panic.

func TestMCPHandler_OpenDatabase_NilArguments(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	// Arguments is nil (zero value of CallToolRequest).
	result, err := handler.OpenDatabase(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("OpenDatabase returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for nil Arguments")
	}
	if !containsString(textFromResult(t, result), "missing or invalid 'path'") {
		t.Errorf("Unexpected message: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_OpenDatabase_NonMapArguments(t *testing.T) {
	handler, cleanup := setupHandlerNoDB(t)
	defer cleanup()

	result, err := handler.OpenDatabase(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: "not-a-map"},
	})
	if err != nil {
		t.Fatalf("OpenDatabase returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for non-map Arguments")
	}
}

func TestMCPHandler_Query_NilArguments(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	result, err := handler.Query(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("Query returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for nil Arguments")
	}
	if !containsString(textFromResult(t, result), "sql query parameter is required") {
		t.Errorf("Unexpected message: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_Query_NonMapArguments(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	result, err := handler.Query(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: 42},
	})
	if err != nil {
		t.Fatalf("Query returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for non-map Arguments")
	}
}

func TestMCPHandler_Execute_NilArguments(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	result, err := handler.Execute(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("Execute returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for nil Arguments")
	}
	if !containsString(textFromResult(t, result), "missing or invalid 'sql'") {
		t.Errorf("Unexpected message: %s", textFromResult(t, result))
	}
}

func TestMCPHandler_Execute_NonMapArguments(t *testing.T) {
	handler, cleanup := setupTestMCPHandler(t)
	defer cleanup()

	result, err := handler.Execute(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: true},
	})
	if err != nil {
		t.Fatalf("Execute returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError=true for non-map Arguments")
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func containsString(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
