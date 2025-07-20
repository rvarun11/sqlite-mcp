package config

import (
	"errors"
	"github.com/spf13/cobra"
	"os"
)

type Config struct {
	DatabasePath string
	Debug        bool
}

func NewConfig(cmd *cobra.Command) (*Config, error) {
	dbPath, _ := cmd.Flags().GetString("database")
	if dbPath == "" {
		return nil, errors.New("database path is required")
	}

	if err := validateDatabasePath(dbPath); err != nil {
		return nil, err
	}

	debug, _ := cmd.Flags().GetBool("debug")

	return &Config{
		DatabasePath: dbPath,
		Debug:        debug,
	}, nil
}

func validateDatabasePath(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dir := dbPath[:len(dbPath)-len(dbPath[findLastSlash(dbPath):])]
		if dir == "" {
			dir = "."
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return errors.New("database directory does not exist")
		}
	}
	return nil
}

func findLastSlash(path string) int {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return i
		}
	}
	return -1
}
