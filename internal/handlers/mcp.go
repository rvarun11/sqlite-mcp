package handlers

import (
	"context"
	"github.com/rvarun11/sqlite-mcp/internal/database"
	"github.com/rvarun11/sqlite-mcp/internal/models"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

type MCPHandler struct {
	db     *database.SQLiteDB
	logger *zap.SugaredLogger
}

func NewMCPHandler(db *database.SQLiteDB, logger *zap.SugaredLogger) *MCPHandler {
	return &MCPHandler{
		db:     db,
		logger: logger,
	}
}

func (h *MCPHandler) GetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling listTables request")

	tables, err := h.db.GetSchema()
	if err != nil {
		h.logger.Error("Failed to list tables", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Failed to retrieve table information. Please check your database connection.",
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: formatTablesResponse(tables),
			},
		},
	}, nil
}

func (h *MCPHandler) QueryDatabase(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling queryDatabase request")

	sql, ok := request.Params.Arguments.(map[string]any)["sql"].(string)
	if !ok || sql == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "SQL query parameter is required",
				},
			},
		}, nil
	}

	result, err := h.db.Query(sql)
	if err != nil {
		h.logger.Error("Query execution failed: ", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Query execution failed. Please check your SQL syntax and try again.",
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: formatQueryResponse(result),
			},
		},
	}, nil
}

func (h *MCPHandler) ExecuteDatabase(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling executeDatabase request")

	sql, ok := request.Params.Arguments.(map[string]any)["sql"].(string)
	if !ok {
		// handle error - sql argument missing or not a string
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Missing or invalid 'sql' argument",
				},
			},
		}, nil
	}

	result, err := h.db.Execute(sql)
	if err != nil {
		h.logger.Error("Statement execution failed: ", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Statement execution failed. Please check your SQL syntax and try again.",
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: formatExecuteResponse(result),
			},
		},
	}, nil
}

// Helper functions for formatting responses
func formatTablesResponse(tables []models.Table) string {
	if len(tables) == 0 {
		return "No tables found in the database."
	}

	response := "Database Tables:\n\n"
	for _, table := range tables {
		response += "Table: " + table.Name + "\n"
		// ... existing column and index formatting ...

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
	response := "Query Results:\n"
	response += "Columns: " + joinStrings(result.Columns, ", ") + "\n"
	response += "Row Count: " + intToString(result.Count) + "\n\n"

	if result.Count > 0 {
		response += "Data:\n"
		for i, row := range result.Rows {
			if i >= 10 { // Limit display to first 10 rows
				response += "... (showing first 10 rows)\n"
				break
			}
			response += "Row " + intToString(i+1) + ": "
			for j, col := range result.Columns {
				if j > 0 {
					response += ", "
				}
				response += col + "=" + interfaceToString(row[col])
			}
			response += "\n"
		}
	}

	return response
}

func formatExecuteResponse(result *models.ExecuteResult) string {
	response := "Execution Result:\n"
	response += "Rows Affected: " + int64ToString(result.RowsAffected) + "\n"
	if result.LastInsertId > 0 {
		response += "Last Insert ID: " + int64ToString(result.LastInsertId) + "\n"
	}
	response += "Message: " + result.Message + "\n"
	return response
}

// Helper utility functions
func joinStrings(strs []string, separator string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += separator + strs[i]
	}
	return result
}

func intToString(n int) string {
	return int64ToString(int64(n))
}

func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	digits := ""
	for n > 0 {
		digits = string(rune('0'+(n%10))) + digits
		n /= 10
	}

	if negative {
		digits = "-" + digits
	}

	return digits
}

func interfaceToString(v any) string {
	if v == nil {
		return "<NULL>"
	}

	switch val := v.(type) {
	case string:
		return val
	case int:
		return intToString(val)
	case int64:
		return int64ToString(val)
	case float64:
		return floatToString(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return "<unknown>"
	}
}

func floatToString(f float64) string {
	// Simple float to string conversion
	if f == float64(int64(f)) {
		return int64ToString(int64(f))
	}
	return "<float>"
}
