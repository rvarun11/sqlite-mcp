package config

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"
	"os"
)

type Config struct {
	DatabasePath string // Empty string means no database at startup
	Debug        bool
}

func NewConfig(cmd *cobra.Command) (*Config, error) {
	dbPath, _ := cmd.Flags().GetString("database")

	// Only validate the path when one is actually provided.
	if dbPath != "" {
		if err := validateDatabasePath(dbPath); err != nil {
			return nil, err
		}
	}

	debug, _ := cmd.Flags().GetBool("debug")

	return &Config{
		DatabasePath: dbPath,
		Debug:        debug,
	}, nil
}

func validateDatabasePath(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dir := filepath.Dir(dbPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return errors.New("database directory does not exist")
		}
	}
	return nil
}
