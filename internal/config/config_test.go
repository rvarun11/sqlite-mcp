package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateDatabasePath_RelativeNoSlash is the regression test for the
// panic caused by a bare filename such as "foo.db" (no directory separator).
// findLastSlash returned -1, making dbPath[-1:] panic with a slice bounds error.
func TestValidateDatabasePath_RelativeNoSlash(t *testing.T) {
	// "foo.db" does not exist but its implicit directory (".") does — should
	// return nil (the file may be created later; we only reject missing dirs).
	if err := validateDatabasePath("foo.db"); err != nil {
		t.Errorf("expected no error for bare filename in cwd, got: %v", err)
	}
}

func TestValidateDatabasePath_ExistingFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "cfg_test_*.db")
	if err != nil {
		t.Fatalf("could not create temp file: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	if err := validateDatabasePath(tmp.Name()); err != nil {
		t.Errorf("expected no error for existing file, got: %v", err)
	}
}

func TestValidateDatabasePath_MissingDirectory(t *testing.T) {
	path := filepath.Join(os.TempDir(), "nonexistent_dir_xyz", "foo.db")
	err := validateDatabasePath(path)
	if err == nil {
		t.Error("expected an error for a path whose directory does not exist, got nil")
	}
}

func TestValidateDatabasePath_RelativeWithSlash(t *testing.T) {
	// sub/foo.db — "sub" directory doesn't exist, so should error.
	err := validateDatabasePath("sub/foo.db")
	if err == nil {
		t.Error("expected an error for a relative path with a missing parent dir, got nil")
	}
}
