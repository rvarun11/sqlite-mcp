package handlers

import (
	"context"
	"fmt"
	"github.com/rvarun11/sqlite-mcp/internal/models"
	"github.com/rvarun11/sqlite-mcp/internal/repository"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

type MCPHandler struct {
	repo   *repository.SQLiteDB
	logger *zap.SugaredLogger
}

func NewMCPHandler(repo *repository.SQLiteDB, logger *zap.SugaredLogger) *MCPHandler {
	return &MCPHandler{
		repo:   repo,
		logger: logger,
	}
}

func (h *MCPHandler) GetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	h.logger.Info("Handling listTables request")

	tables, err := h.repo.GetSchema()
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

func (h *MCPHandler) Query(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	result, err := h.repo.Query(sql)
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

func (h *MCPHandler) Execute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	result, err := h.repo.Execute(sql)
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
