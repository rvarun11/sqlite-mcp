package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"

	"github.com/rvarun11/sqlite-mcp/internal/models"

	"go.uber.org/zap"
)

var _ Repository = (*SQLiteDB)(nil)

type SQLiteDB struct {
	mu     sync.RWMutex
	db     *sql.DB
	logger *zap.SugaredLogger
}

// NewSQLiteDB creates an SQLiteDB without opening a database.
// Call Open to connect to a specific file.
func NewSQLiteDB(logger *zap.SugaredLogger) *SQLiteDB {
	return &SQLiteDB{logger: logger}
}

// Open connects to the SQLite database at dbPath, replacing any existing
// connection. The path must point to an existing file.
func (s *SQLiteDB) Open(dbPath string) error {
	// Validate that the file exists — we do not create new databases via this
	// method (creation is only allowed through the CLI flag at startup).
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database file does not exist: %s", dbPath)
	}

	newDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := newDB.Ping(); err != nil {
		newDB.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	newDB.SetMaxOpenConns(25)
	newDB.SetMaxIdleConns(5)

	s.mu.Lock()
	old := s.db
	s.db = newDB
	s.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			s.logger.Warnf("Failed to close previous database connection: %v", err)
		}
	}

	s.logger.Infof("Connected to SQLite database: %v", dbPath)
	return nil
}

// NewSQLiteDBFromPath is a convenience constructor that opens the database
// immediately. It is used during startup when --database is provided.
func NewSQLiteDBFromPath(dbPath string, logger *zap.SugaredLogger) (*SQLiteDB, error) {
	s := NewSQLiteDB(logger)
	if err := s.Open(dbPath); err != nil {
		return nil, err
	}
	return s, nil
}

// ErrNoDatabase is an error returned by handlers if no database is open.
var ErrNoDatabase = errors.New("no database is open")

func (s *SQLiteDB) GetSchema() ([]models.Table, error) {
	s.logger.Debug("Get database schema")

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, ErrNoDatabase
	}

	var tableNames []string

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		s.logger.Errorf("Failed to retrieve table names: %v", err)
		return nil, fmt.Errorf("failed to retrieve table information")
	}
	defer rows.Close()

	// Scan the results
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			s.logger.Errorf("Failed to scan table name: %v", err)
			continue
		}
		tableNames = append(tableNames, tableName)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		s.logger.Errorf("Error during table name iteration: %v", err)
		return nil, fmt.Errorf("failed to retrieve table information")
	}

	tables := make([]models.Table, 0, len(tableNames))

	for _, tableName := range tableNames {
		tableInfo, err := s.getTableInfo(db, tableName)
		if err != nil {
			s.logger.Errorf("Failed to get table info for table %s: %v", tableName, err)
			continue
		}
		tables = append(tables, *tableInfo)
	}

	s.logger.Infof("Successfully retrieved table information, table_count: %d", len(tables))
	return tables, nil
}

func (s *SQLiteDB) getTableInfo(db *sql.DB, tableName string) (*models.Table, error) {
	// Get column information
	var columns []models.Column
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		if err != nil {
			return nil, err
		}

		column := models.Column{
			Name:       name,
			Type:       dataType,
			NotNull:    notNull == 1,
			PrimaryKey: pk == 1,
		}

		if defaultValue.Valid {
			column.DefaultValue = &defaultValue.String
		}

		columns = append(columns, column)
	}

	// Get index information
	var indexes []string
	indexRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer indexRows.Close()

	for indexRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		err := indexRows.Scan(&seq, &name, &unique, &origin, &partial)
		if err != nil {
			continue
		}

		if !strings.HasPrefix(name, "sqlite_autoindex") {
			indexes = append(indexes, name)
		}
	}

	var foreignKeys []models.ForeignKey
	fkRows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer fkRows.Close()

	for fkRows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string

		err := fkRows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match)
		if err != nil {
			return nil, err
		}

		foreignKey := models.ForeignKey{
			ID:       id,
			Seq:      seq,
			Table:    table,
			From:     from,
			To:       to,
			OnUpdate: onUpdate,
			OnDelete: onDelete,
			Match:    match,
		}

		foreignKeys = append(foreignKeys, foreignKey)
	}

	return &models.Table{
		Name:        tableName,
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: foreignKeys,
	}, nil
}

func (s *SQLiteDB) Query(sqlQuery string) (*models.QueryResult, error) {
	s.logger.Debugf("Executing query: %s", sanitizeQuery(sqlQuery))

	if !isSelectQuery(sqlQuery) {
		return nil, fmt.Errorf("only SELECT queries are allowed for query operations")
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, ErrNoDatabase
	}

	rows, err := db.Query(sqlQuery)
	if err != nil {
		s.logger.Errorf("Query execution failed: %v", err)
		return nil, fmt.Errorf("query execution failed")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve column information")
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row data")
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	result := &models.QueryResult{
		Columns: columns,
		Rows:    results,
		Count:   len(results),
	}

	s.logger.Infof("Query executed successfully, rows_returned: %d", len(results))
	return result, nil
}

func (s *SQLiteDB) Execute(sqlQuery string) (*models.ExecuteResult, error) {
	s.logger.Debugf("Executing statement: %s", sanitizeQuery(sqlQuery))

	if isSelectQuery(sqlQuery) {
		return nil, fmt.Errorf("SELECT queries should use the query operation instead")
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, ErrNoDatabase
	}

	result, err := db.Exec(sqlQuery)
	if err != nil {
		s.logger.Errorf("Statement execution failed: %v", err)
		return nil, fmt.Errorf("statement execution failed")
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertId, _ := result.LastInsertId()

	executeResult := &models.ExecuteResult{
		RowsAffected: rowsAffected,
		LastInsertId: lastInsertId,
		Message:      fmt.Sprintf("Statement executed successfully, %d rows affected", rowsAffected),
	}

	s.logger.Infof("Statement executed successfully, rows_affected: %d, last_insert_id: %d", rowsAffected, lastInsertId)

	return executeResult, nil
}

func (s *SQLiteDB) Close() error {
	s.mu.Lock()
	db := s.db
	s.db = nil
	s.mu.Unlock()

	if db != nil {
		return db.Close()
	}
	return nil
}

// Helper functions
func isSelectQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(trimmed, "SELECT") ||
		strings.HasPrefix(trimmed, "WITH") ||
		strings.HasPrefix(trimmed, "EXPLAIN")
}

// TODO: To be improved with more complex sanitization logic
// sanitizeQuery sanitizes the SQL query string
func sanitizeQuery(query string) string {
	if len(query) > 100 {
		return query[:97] + "..."
	}
	return query
}
