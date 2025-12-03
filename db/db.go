package db

import (
	"github.com/ContainerSolutions/todo-app/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// AutoMigrateFunc is the function used for auto-migration (can be mocked for testing)
var AutoMigrateFunc = func(db *gorm.DB) error {
	return db.AutoMigrate(&models.User{}, &models.Todo{})
}

// InitDB initializes the database connection and runs migrations
func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	// Auto-migrate the schema
	err = AutoMigrateFunc(DB)
	if err != nil {
		return err
	}

	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// SetDB sets the database instance (useful for testing)
func SetDB(database *gorm.DB) {
	DB = database
}
