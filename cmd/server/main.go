package main

import (
	"fmt"
	"github.com/rvarun11/sqlite-mcp/internal/config"
	"github.com/rvarun11/sqlite-mcp/internal/handlers"
	"github.com/rvarun11/sqlite-mcp/internal/logger"
	"github.com/rvarun11/sqlite-mcp/internal/repository"
	"os"

	"context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os/signal"
	"syscall"
)

var dbPath string

func main() {
	var rootCmd = &cobra.Command{
		Use:   "sqlite-mcp",
		Short: "SQLite MCP Server - A Model Context Protocol server for SQLite operations",
		Long:  `SQLite MCP Server provides a standardized interface for SQLite database operations through the Model Context Protocol (MCP). It supports schema introspection, query execution, and database modifications.`,
		Run:   runServer,
	}

	rootCmd.Flags().StringVarP(&dbPath, "database", "d", "", "Path to SQLite database file (required)")
	rootCmd.Flags().Bool("debug", false, "Enable debug mode")

	err := rootCmd.MarkFlagRequired("database")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marking database flag as required: %v\n", err)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Initialize configuration
	cfg, err := config.NewConfig(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logger.NewLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer syncLogger(logger)

	logger.Infof("Starting SQLite MCP Server: %v", dbPath)

	// Initialize database
	repo, err := repository.NewSQLiteDB(cfg.DatabasePath, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()

	// Initialize MCP handler
	mcpHandler := handlers.NewMCPHandler(repo, logger)

	mcpServer := server.NewMCPServer(
		"sqlite-mcp",
		"1.0.0",
	)

	// Get Schema Tool - No parameters needed
	listTablesTool := mcp.NewTool("get_schema",
		mcp.WithDescription("List all tables in the SQLite database with their schema information including columns, types, constraints, and indexes"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
	mcpServer.AddTool(listTablesTool, mcpHandler.GetSchema)

	// Query Database Tool
	queryDatabaseTool := mcp.NewTool("query",
		mcp.WithDescription("Execute SELECT queries against the SQLite database. Only SELECT, WITH, and EXPLAIN queries are allowed."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL SELECT query to execute"),
			mcp.MinLength(1),
			mcp.MaxLength(10000),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
	mcpServer.AddTool(queryDatabaseTool, mcpHandler.Query)

	// Execute Database Tool
	executeDatabaseTool := mcp.NewTool("execute",
		mcp.WithDescription("Execute DDL/DML operations (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, etc.) against the SQLite database. SELECT queries are not allowed - use queryDatabase instead."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL statement to execute (non-SELECT operations only)"),
			mcp.MinLength(1),
			mcp.MaxLength(10000),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
	)
	mcpServer.AddTool(executeDatabaseTool, mcpHandler.Execute)

	//Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start server
	// TODO: Look into how you want to start the server
	logger.Info("SQLite MCP Server started successfully")
	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}

	logger.Info("SQLite MCP Server stopped")
}

func syncLogger(logger *zap.SugaredLogger) {
	if err := logger.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}
