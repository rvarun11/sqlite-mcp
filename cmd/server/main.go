package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rvarun11/sqlite-mcp/internal/config"
	"github.com/rvarun11/sqlite-mcp/internal/handlers"
	"github.com/rvarun11/sqlite-mcp/internal/logger"
	"github.com/rvarun11/sqlite-mcp/internal/repository"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var dbPath string

func main() {
	var rootCmd = &cobra.Command{
		Use:   "sqlite-mcp",
		Short: "SQLite MCP Server - A Model Context Protocol server for SQLite operations",
		Long:  `SQLite MCP Server provides a standardized interface for SQLite database operations through the Model Context Protocol (MCP). It supports schema introspection, query execution, and database modifications.`,
		Run:   runServer,
	}

	rootCmd.Flags().StringVarP(&dbPath, "database", "d", "", "Path to SQLite database file (optional; if omitted, call open_database before using other tools)")
	rootCmd.Flags().Bool("debug", false, "Enable debug mode")

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
	log, err := logger.NewLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer syncLogger(log)

	// Initialize repository (connection opened below if --database was provided)
	repo := repository.NewSQLiteDB(log)
	defer repo.Close()

	if cfg.DatabasePath != "" {
		log.Infof("Opening database at startup: %s", cfg.DatabasePath)
		if err := repo.Open(cfg.DatabasePath); err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
	} else {
		log.Info("No database specified at startup — awaiting open_database tool call")
	}

	// Initialize MCP handler
	mcpHandler := handlers.NewMCPHandler(repo, log)

	mcpServer := server.NewMCPServer(
		"sqlite-mcp",
		"1.0.0",
	)

	// Open Database Tool
	openDatabaseTool := mcp.NewTool("open_database",
		mcp.WithDescription("Open a SQLite database file, closing any previously open database. Must be called before using other tools when no --database flag was provided at startup."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute path to the SQLite database file to open. The file must already exist."),
			mcp.MinLength(1),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
	)
	mcpServer.AddTool(openDatabaseTool, mcpHandler.OpenDatabase)

	// Get Schema Tool
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
		mcp.WithDescription("Execute DDL/DML operations (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, etc.) against the SQLite database. SELECT queries are not allowed - use query instead."),
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

	// Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start server
	// TODO: Look into alternatives for transport layer (sse, streamable-http)
	log.Info("SQLite MCP Server started successfully")
	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}

	log.Info("SQLite MCP Server stopped")
}

func syncLogger(logger *zap.SugaredLogger) {
	if err := logger.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}
