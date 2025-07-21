package repository

import "github.com/rvarun11/sqlite-mcp/internal/models"

type Repository interface {
	GetSchema() ([]models.Table, error)
	Query(sqlQuery string) (*models.QueryResult, error)
	Execute(sqlQuery string) (*models.ExecuteResult, error)
	Close() error
}
