package database

import (
	"database/sql"
	"fmt"
	"github.com/rvarun11/sqlite-mcp/internal/models"
	"strings"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	// "gorm.io/gorm/logger"
)

type SQLiteDB struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	logger *zap.SugaredLogger
}

func NewSQLiteDB(dbPath string, logger *zap.SugaredLogger) (*SQLiteDB, error) {
	// Configure GORM with custom logger
	// gormConfig := &gorm.Config{
	// 	Logger: logger.Default.LogMode(logger.Silent), // We'll use zap for logging
	// }

	db, err := gorm.Open(sqlite.Open(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	logger.Infof("Connected to SQLite database: %v", dbPath)

	return &SQLiteDB{
		db:     db,
		sqlDB:  sqlDB,
		logger: logger,
	}, nil
}

func (s *SQLiteDB) GetSchema() ([]models.Table, error) {
	s.logger.Debug("Get database schema")

	var tableNames []string
	err := s.db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableNames).Error
	if err != nil {
		s.logger.Error("Failed to retrieve table names", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve table information")
	}

	tables := make([]models.Table, 0, len(tableNames))

	for _, tableName := range tableNames {
		tableInfo, err := s.getTableInfo(tableName)
		if err != nil {
			s.logger.Error("Failed to get table info", zap.String("table", tableName), zap.Error(err))
			continue
		}
		tables = append(tables, *tableInfo)
	}

	s.logger.Info("Successfully retrieved table information", zap.Int("table_count", len(tables)))
	return tables, nil
}

func (s *SQLiteDB) getTableInfo(tableName string) (*models.Table, error) {
	// Get column information
	var columns []models.Column
	rows, err := s.sqlDB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
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
	indexRows, err := s.sqlDB.Query(fmt.Sprintf("PRAGMA index_list(%s)", tableName))
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
	fkRows, err := s.sqlDB.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName))
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
	s.logger.Debug("Executing query", zap.String("query", sanitizeQuery(sqlQuery)))

	if !isSelectQuery(sqlQuery) {
		return nil, fmt.Errorf("only SELECT queries are allowed for query operations")
	}

	rows, err := s.sqlDB.Query(sqlQuery)
	if err != nil {
		s.logger.Error("Query execution failed", zap.Error(err))
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

	s.logger.Info("Query executed successfully", zap.Int("rows_returned", len(results)))
	return result, nil
}

func (s *SQLiteDB) Execute(sqlQuery string) (*models.ExecuteResult, error) {
	s.logger.Debug("Executing statement", zap.String("query", sanitizeQuery(sqlQuery)))

	if isSelectQuery(sqlQuery) {
		return nil, fmt.Errorf("SELECT queries should use the query operation instead")
	}

	result, err := s.sqlDB.Exec(sqlQuery)
	if err != nil {
		s.logger.Error("Statement execution failed", zap.Error(err))
		return nil, fmt.Errorf("statement execution failed")
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertId, _ := result.LastInsertId()

	executeResult := &models.ExecuteResult{
		RowsAffected: rowsAffected,
		LastInsertId: lastInsertId,
		Message:      fmt.Sprintf("Statement executed successfully, %d rows affected", rowsAffected),
	}

	s.logger.Info("Statement executed successfully",
		zap.Int64("rows_affected", rowsAffected),
		zap.Int64("last_insert_id", lastInsertId))

	return executeResult, nil
}

func (s *SQLiteDB) Close() error {
	if s.sqlDB != nil {
		return s.sqlDB.Close()
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

func sanitizeQuery(query string) string {
	// Basic sanitization for logging - remove potential sensitive data patterns
	if len(query) > 100 {
		return query[:97] + "..."
	}
	return query
}
