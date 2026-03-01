package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rvarun11/sqlite-mcp/internal/models"
	"github.com/rvarun11/sqlite-mcp/internal/repository"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

type MCPHandler struct {
	repo   repository.Repository
	logger *zap.SugaredLogger

	// gate is held as a read lock by every in-flight get_schema/query/execute
	// call and as a write lock by open_database. This guarantees:
	//   - open_database waits for all active data operations to finish before
	//     swapping the connection (write lock blocks until all readers release).
	//   - No new data operation can start while a swap is in progress (read
	//     lock blocks while the write lock is held).
	//   - Concurrent open_database calls are serialised automatically by the
	//     write lock; the atomic flag below lets us reject the second one
	//     immediately instead of queuing it.
	gate sync.RWMutex

	// opening is 1 while an open_database call holds (or is waiting for) the
	// write lock, so a second concurrent open_database can be rejected fast.
	opening atomic.Int32
}

func NewMCPHandler(repo repository.Repository, logger *zap.SugaredLogger) *MCPHandler {
	return &MCPHandler{
		repo:   repo,
		logger: logger,
	}
}

// argsMap safely extracts the Arguments map from a CallToolRequest.
// Returns nil if Arguments is nil or not a map[string]any.
func argsMap(request mcp.CallToolRequest) map[string]any {
	m, _ := request.Params.Arguments.(map[string]any)
	return m
}

func (h *MCPHandler) OpenDatabase(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling open_database request")

	args := argsMap(request)
	path, _ := args["path"].(string)
	if path == "" {
		return mcp.NewToolResultError("Missing or invalid 'path' argument"), nil
	}

	// Reject a second concurrent open_database call immediately.
	if !h.opening.CompareAndSwap(0, 1) {
		return mcp.NewToolResultError("A database open operation is already in progress. Please try again later."), nil
	}
	defer h.opening.Store(0)

	// Acquire the write lock. This blocks until every in-flight data operation
	// releases its read lock, and prevents new data operations from starting.
	h.gate.Lock()
	defer h.gate.Unlock()

	if err := h.repo.Open(path); err != nil {
		h.logger.Errorf("Failed to open database %q: %v", path, err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to open database: %v", err)), nil
	}

	// Fetch schema so the caller immediately knows what is in the new database.
	tables, err := h.repo.GetSchema()
	if err != nil {
		h.logger.Errorf("Failed to retrieve schema after opening database: %v", err)
		return mcp.NewToolResultError("Database opened but failed to retrieve schema."), nil
	}

	response := fmt.Sprintf("Opened database at %s\n\n", path) + formatTablesResponse(tables)
	return mcp.NewToolResultText(response), nil
}

// mcpErrNoDatabaseOpen is returned by tools when no database is open.
var mcpErrNoDatabaseOpen = mcp.NewToolResultError("No database is open. Call open_database first.")

func (h *MCPHandler) GetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling listTables request")

	h.gate.RLock()
	defer h.gate.RUnlock()

	tables, err := h.repo.GetSchema()
	if err != nil {
		h.logger.Error("Failed to list tables", err)
		if errors.Is(err, repository.ErrNoDatabase) {
			return mcpErrNoDatabaseOpen, nil
		}
		return mcp.NewToolResultError("Failed to retrieve table information. Please check your database connection."), nil
	}

	return mcp.NewToolResultText(formatTablesResponse(tables)), nil
}

func (h *MCPHandler) Query(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling queryDatabase request")

	h.gate.RLock()
	defer h.gate.RUnlock()

	args := argsMap(request)
	sql, _ := args["sql"].(string)
	if sql == "" {
		return mcp.NewToolResultError("SQL query parameter is required"), nil
	}

	result, err := h.repo.Query(sql)
	if err != nil {
		h.logger.Error("Query execution failed: ", err)
		if errors.Is(err, repository.ErrNoDatabase) {
			return mcpErrNoDatabaseOpen, nil
		}
		return mcp.NewToolResultError("Query execution failed. Please check your SQL syntax and try again."), nil
	}

	return mcp.NewToolResultText(formatQueryResponse(result)), nil
}

func (h *MCPHandler) Execute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling executeDatabase request")

	h.gate.RLock()
	defer h.gate.RUnlock()

	args := argsMap(request)
	sql, ok := args["sql"].(string)
	if !ok {
		return mcp.NewToolResultError("Missing or invalid 'sql' argument"), nil
	}

	result, err := h.repo.Execute(sql)
	if err != nil {
		h.logger.Error("Statement execution failed: ", err)
		if errors.Is(err, repository.ErrNoDatabase) {
			return mcpErrNoDatabaseOpen, nil
		}
		return mcp.NewToolResultError("Statement execution failed. Please check your SQL syntax and try again."), nil
	}

	return mcp.NewToolResultText(formatExecuteResponse(result)), nil
}

// Helper functions for formatting responses
func formatTablesResponse(tables []models.Table) string {
	if len(tables) == 0 {
		return "No tables found in the database."
	}

	response := "Database Tables:\n\n"
	for _, table := range tables {
		response += "Table: " + table.Name + "\n"

		if len(table.Columns) > 0 {
			response += "Columns:\n"
			for _, col := range table.Columns {
				response += "  - " + col.Name + " (" + col.Type + ")"
				if col.NotNull {
					response += " NOT NULL"
				}
				if col.PrimaryKey {
					response += " PRIMARY KEY"
				}
				if col.DefaultValue != nil {
					response += " DEFAULT " + *col.DefaultValue
				}
				response += "\n"
			}
		}

		if len(table.Indexes) > 0 {
			response += "Indexes:\n"
			for _, index := range table.Indexes {
				response += "  - " + index + "\n"
			}
		}

		if len(table.ForeignKeys) > 0 {
			response += "Foreign Keys:\n"
			for _, fk := range table.ForeignKeys {
				response += "  - " + fk.From + " -> " + fk.Table + "(" + fk.To + ")"
				if fk.OnDelete != "NO ACTION" {
					response += " ON DELETE " + fk.OnDelete
				}
				if fk.OnUpdate != "NO ACTION" {
					response += " ON UPDATE " + fk.OnUpdate
				}
				response += "\n"
			}
		}
		response += "\n"
	}

	return response
}

func formatQueryResponse(result *models.QueryResult) string {
	response := fmt.Sprintf("Query Results:\nColumns: %s\nRow Count: %d\n\n",
		strings.Join(result.Columns, ", "),
		result.Count)

	if result.Count > 0 {
		response += "Data:\n"
		for i, row := range result.Rows {
			if i >= 10 { // Limit display to first 10 rows
				response += "... (showing first 10 rows)\n"
				break
			}

			response += fmt.Sprintf("Row %d: ", i+1)

			var pairs []string
			for _, col := range result.Columns {
				value := row[col]
				if value == nil {
					value = "<NULL>"
				}
				pairs = append(pairs, fmt.Sprintf("%s=%v", col, value))
			}
			response += strings.Join(pairs, ", ") + "\n"
		}
	}

	return response
}

func formatExecuteResponse(result *models.ExecuteResult) string {
	response := "Execution Result:\n"
	response += fmt.Sprintf("Rows Affected: %d\n", result.RowsAffected)
	if result.LastInsertId > 0 {
		response += fmt.Sprintf("Last Insert ID: %d\n", result.LastInsertId)
	}
	response += "Message: " + result.Message + "\n"
	return response
}
