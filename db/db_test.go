package db

import (
	"testing"

	"gorm.io/gorm"
)

// Test_DB_P_001_InitDBSuccess verifies database initialization works
func Test_DB_P_001_InitDBSuccess(t *testing.T) {
	err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	if DB == nil {
		t.Error("DB should not be nil after initialization")
	}
}

// Test_DB_P_002_GetDBReturnsInstance verifies GetDB returns the database instance
func Test_DB_P_002_GetDBReturnsInstance(t *testing.T) {
	err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	db := GetDB()
	if db == nil {
		t.Error("GetDB should return non-nil database instance")
	}

	if db != DB {
		t.Error("GetDB should return the same instance as DB")
	}
}

// Test_DB_N_001_InitDBInvalidPath verifies database initialization fails with invalid path
func Test_DB_N_001_InitDBInvalidPath(t *testing.T) {
	// Try to create a database in a non-existent directory
	err := InitDB("/nonexistent/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Expected error for invalid database path")
	}
}

// Test_DB_P_003_SetDB verifies SetDB sets the database instance
func Test_DB_P_003_SetDB(t *testing.T) {
	err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	originalDB := GetDB()
	SetDB(nil)

	if GetDB() != nil {
		t.Error("SetDB should set DB to nil")
	}

	// Restore
	SetDB(originalDB)
	if GetDB() != originalDB {
		t.Error("SetDB should restore original DB")
	}
}

// Test_DB_N_002_InitDBAutoMigrateError verifies InitDB handles AutoMigrate error
func Test_DB_N_002_InitDBAutoMigrateError(t *testing.T) {
	// Mock AutoMigrateFunc to return an error
	originalFunc := AutoMigrateFunc
	AutoMigrateFunc = func(db *gorm.DB) error {
		return gorm.ErrInvalidDB
	}
	defer func() { AutoMigrateFunc = originalFunc }()

	err := InitDB(":memory:")
	if err == nil {
		t.Error("Expected error for AutoMigrate failure")
	}
}
