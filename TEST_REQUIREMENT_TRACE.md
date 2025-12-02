# Leo App - Requirements Traceability Matrix

## 1. Overview

This document provides traceability from software requirements to test cases, ensuring complete verification coverage as required by DO-178C.

## 2. Requirements Summary

| ID | Requirement | Status |
|----|-------------|--------|
| REQ01 | Users should be able to LOGIN | Implemented |
| REQ02 | Users should be able to create new TODOs | Implemented |
| REQ03 | Users should be able to see all todo lists | Implemented |
| REQ04 | Users should NOT be able to modify/delete TODOs they did not create | Implemented |

## 3. Traceability Matrix

### REQ01: User Login

| Test ID | Test Name | Type | Description |
|---------|-----------|------|-------------|
| REQ01_P_001 | Test_REQ01_P_001_LoginSuccess | Positive | Verify user can login with valid credentials |
| REQ01_P_002 | Test_REQ01_P_002_RegisterSuccess | Positive | Verify user can register a new account |
| REQ01_N_001 | Test_REQ01_N_001_LoginInvalidPassword | Negative | Verify login fails with wrong password |
| REQ01_N_002 | Test_REQ01_N_002_LoginNonexistentUser | Negative | Verify login fails for non-existent user |
| REQ01_N_003 | Test_REQ01_N_003_RegisterDuplicateUsername | Negative | Verify registration fails for duplicate username |
| REQ01_E_001 | Test_REQ01_E_001_TokenExpiry | Edge | Verify expired tokens are rejected |

### REQ02: Create TODOs

| Test ID | Test Name | Type | Description |
|---------|-----------|------|-------------|
| REQ02_P_001 | Test_REQ02_P_001_CreateTodoSuccess | Positive | Verify authenticated user can create a todo |
| REQ02_P_002 | Test_REQ02_P_002_CreateTodoWithDescription | Positive | Verify todo can be created with description |
| REQ02_N_001 | Test_REQ02_N_001_CreateTodoUnauthenticated | Negative | Verify unauthenticated user cannot create todo |
| REQ02_N_002 | Test_REQ02_N_002_CreateTodoEmptyTitle | Negative | Verify todo creation fails without title |
| REQ02_E_001 | Test_REQ02_E_001_CreateTodoMaxLength | Edge | Verify todo with max length title succeeds |

### REQ03: View TODOs

| Test ID | Test Name | Type | Description |
|---------|-----------|------|-------------|
| REQ03_P_001 | Test_REQ03_P_001_ListAllTodos | Positive | Verify all todos are returned |
| REQ03_P_002 | Test_REQ03_P_002_GetTodoById | Positive | Verify single todo can be retrieved |
| REQ03_P_003 | Test_REQ03_P_003_ListTodosUnauthenticated | Positive | Verify unauthenticated users can view todos |
| REQ03_N_001 | Test_REQ03_N_001_GetNonexistentTodo | Negative | Verify 404 for non-existent todo |
| REQ03_E_001 | Test_REQ03_E_001_ListEmptyTodos | Edge | Verify empty list returned when no todos |

### REQ04: Ownership Restrictions

| Test ID | Test Name | Type | Description |
|---------|-----------|------|-------------|
| REQ04_P_001 | Test_REQ04_P_001_UpdateOwnTodo | Positive | Verify user can update their own todo |
| REQ04_P_002 | Test_REQ04_P_002_DeleteOwnTodo | Positive | Verify user can delete their own todo |
| REQ04_N_001 | Test_REQ04_N_001_UpdateOtherUserTodo | Negative | Verify user cannot update another user's todo |
| REQ04_N_002 | Test_REQ04_N_002_DeleteOtherUserTodo | Negative | Verify user cannot delete another user's todo |
| REQ04_N_003 | Test_REQ04_N_003_UpdateTodoUnauthenticated | Negative | Verify unauthenticated user cannot update |
| REQ04_N_004 | Test_REQ04_N_004_DeleteTodoUnauthenticated | Negative | Verify unauthenticated user cannot delete |

## 4. Coverage Summary

| Requirement | Total Tests | Positive | Negative | Edge |
|-------------|-------------|----------|----------|------|
| REQ01 | 6 | 2 | 3 | 1 |
| REQ02 | 5 | 2 | 2 | 1 |
| REQ03 | 5 | 3 | 1 | 1 |
| REQ04 | 6 | 2 | 4 | 0 |
| **Total** | **22** | **9** | **10** | **3** |

## 5. Test Execution

Tests are executed using:
```bash
go test -v ./...
```

Coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## 6. Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2025-12-02 | Container Solutions | Initial traceability matrix |
