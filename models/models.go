package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
// REQ01: Users should be able to LOGIN
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Username  string         `gorm:"uniqueIndex;not null;size:255" json:"username"`
	Password  string         `gorm:"not null" json:"-"` // Password hash, never exposed in JSON
	Email     *string        `gorm:"uniqueIndex;size:320" json:"email,omitempty"`

	OIDCIssuer   *string `gorm:"column:oidc_issuer;size:512;uniqueIndex:idx_oidc_identity" json:"-"`
	OIDCSubject  *string `gorm:"column:oidc_subject;size:255;uniqueIndex:idx_oidc_identity" json:"-"`
	AuthProvider string  `gorm:"not null;size:32;default:password" json:"auth_provider"`
}

// Todo represents a todo item
// REQ02: Users should be able to create new TODOs
// REQ03: Users should be able to see all todo lists
// REQ04: Users should NOT be able to modify/delete TODOs they did not create
type Todo struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Title       string         `gorm:"not null;size:255" json:"title"`
	Description string         `gorm:"size:1000" json:"description"`
	Completed   bool           `gorm:"default:false" json:"completed"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	User        User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// TableName specifies the table name for Todo
func (Todo) TableName() string {
	return "todos"
}
