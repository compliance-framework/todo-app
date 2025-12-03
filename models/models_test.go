package models

import (
	"testing"
)

// Test_Models_P_001_UserTableName verifies User.TableName returns correct table name
func Test_Models_P_001_UserTableName(t *testing.T) {
	user := User{}
	expected := "users"
	if user.TableName() != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, user.TableName())
	}
}

// Test_Models_P_002_TodoTableName verifies Todo.TableName returns correct table name
func Test_Models_P_002_TodoTableName(t *testing.T) {
	todo := Todo{}
	expected := "todos"
	if todo.TableName() != expected {
		t.Errorf("Expected table name '%s', got '%s'", expected, todo.TableName())
	}
}
