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

| Test ID | Test Name | Type | File | Description |
|---------|-----------|------|------|-------------|
| REQ01_P_001 | Test_REQ01_P_001_HashAndCheckPassword | Positive | auth/auth_test.go | Verify password hashing works correctly |
| REQ01_P_002 | Test_REQ01_P_002_GenerateAndValidateToken | Positive | auth/auth_test.go | Verify token generation and validation |
| REQ01_P_003 | Test_REQ01_P_003_AuthMiddlewareValidToken | Positive | auth/auth_test.go | Verify middleware passes valid token |
| REQ01_P_004 | Test_REQ01_P_004_GetUserIDFromContextSuccess | Positive | auth/auth_test.go | Verify user ID extraction from context |
| REQ01_P_005 | Test_REQ01_P_005_LoginSuccess | Positive | handlers/handlers_test.go | Verify user can login with valid credentials |
| REQ01_P_006 | Test_REQ01_P_006_RegisterSuccess | Positive | handlers/handlers_test.go | Verify user can register a new account |
| REQ01_P_007 | Test_REQ01_P_007_AuthConfig | Positive | handlers/handlers_test.go | Verify auth config reports OIDC availability |
| REQ01_P_008 | Test_REQ01_P_008_UpsertOIDCUserCreateAndFind | Positive | handlers/handlers_test.go | Verify OIDC users are created and reused |
| REQ01_P_009 | Test_REQ01_P_009_UpsertOIDCUserAttachByEmail | Positive | handlers/handlers_test.go | Verify OIDC identity attaches by email |
| REQ01_P_010 | Test_REQ01_P_010_OIDCUtilities | Positive | handlers/handlers_test.go | Verify OIDC utility helpers |
| REQ01_P_011 | Test_REQ01_P_011_OIDCLoginRedirect | Positive | handlers/handlers_test.go | Verify OIDC login redirects to provider |
| REQ01_P_012 | Test_REQ01_P_012_OIDCCallbackSuccess | Positive | handlers/handlers_test.go | Verify OIDC callback creates user and returns app JWT |
| REQ01_P_013 | Test_REQ01_P_013_UpsertOIDCUserDoesNotAttachUnverifiedEmail | Positive | handlers/handlers_test.go | Verify unverified OIDC emails do not attach to existing users |
| REQ01_P_014 | Test_REQ01_P_014_AttachOIDCIdentityRefusesRelink | Positive | handlers/handlers_test.go | Verify linked OIDC accounts cannot be re-linked to another identity |
| Auth_P_001 | Test_Auth_P_001_ConfigureJWTSecretFromEnv | Positive | auth/auth_test.go | Verify JWT secret is read from environment |
| Auth_P_002 | Test_Auth_P_002_ConfigureJWTSecretDevelopmentFallback | Positive | auth/auth_test.go | Verify development JWT secret fallback |
| Auth_P_003 | Test_Auth_P_003_IsDevelopmentModeFallbacks | Positive | auth/auth_test.go | Verify development mode detection |
| Auth_P_004 | Test_Auth_P_004_OIDCConfigFromEnv | Positive | auth/auth_test.go | Verify OIDC config is read from environment |
| Auth_P_005 | Test_Auth_P_005_OIDCConfigOAuth2ConfigSuccess | Positive | auth/auth_test.go | Verify OIDC provider discovery configures OAuth2 |
| REQ01_N_001 | Test_REQ01_N_001_CheckWrongPassword | Negative | auth/auth_test.go | Verify wrong password fails check |
| REQ01_N_002 | Test_REQ01_N_002_ValidateInvalidToken | Negative | auth/auth_test.go | Verify invalid token is rejected |
| REQ01_N_003 | Test_REQ01_N_003_AuthMiddlewareNoHeader | Negative | auth/auth_test.go | Verify middleware rejects missing header |
| REQ01_N_004 | Test_REQ01_N_004_AuthMiddlewareInvalidFormat | Negative | auth/auth_test.go | Verify middleware rejects invalid format |
| REQ01_N_005 | Test_REQ01_N_005_AuthMiddlewareInvalidToken | Negative | auth/auth_test.go | Verify middleware rejects invalid token |
| REQ01_N_006 | Test_REQ01_N_006_AuthMiddlewareMalformedHeader | Negative | auth/auth_test.go | Verify middleware rejects malformed header |
| REQ01_N_007 | Test_REQ01_N_007_GetUserIDFromContextMissing | Negative | auth/auth_test.go | Verify missing user ID returns false |
| REQ01_N_008 | Test_REQ01_N_008_GetUserIDFromContextInvalidType | Negative | auth/auth_test.go | Verify invalid user ID type returns false |
| REQ01_N_009 | Test_REQ01_N_009_LoginInvalidPassword | Negative | handlers/handlers_test.go | Verify login fails with wrong password |
| REQ01_N_010 | Test_REQ01_N_010_LoginNonexistentUser | Negative | handlers/handlers_test.go | Verify login fails for non-existent user |
| REQ01_N_011 | Test_REQ01_N_011_RegisterDuplicateUsername | Negative | handlers/handlers_test.go | Verify registration fails for duplicate username |
| REQ01_N_012 | Test_REQ01_N_012_LoginInvalidJSON | Negative | handlers/handlers_test.go | Verify login fails with invalid JSON |
| REQ01_N_013 | Test_REQ01_N_013_RegisterInvalidJSON | Negative | handlers/handlers_test.go | Verify registration fails with invalid JSON |
| REQ01_N_014 | Test_REQ01_N_014_OIDCLoginNotConfigured | Negative | handlers/handlers_test.go | Verify OIDC login fails when disabled |
| REQ01_N_015 | Test_REQ01_N_015_OIDCCallbackInvalidState | Negative | handlers/handlers_test.go | Verify OIDC callback rejects invalid state |
| REQ01_N_016 | Test_REQ01_N_016_OIDCCallbackMissingCode | Negative | handlers/handlers_test.go | Verify OIDC callback requires an auth code |
| REQ01_N_017 | Test_REQ01_N_017_OIDCCallbackTokenExchangeFailed | Negative | handlers/handlers_test.go | Verify OIDC callback handles token exchange failures |
| REQ01_N_018 | Test_REQ01_N_018_OIDCCallbackMissingIDToken | Negative | handlers/handlers_test.go | Verify OIDC callback requires an ID token |
| REQ01_N_019 | Test_REQ01_N_019_OIDCCallbackInvalidIDToken | Negative | handlers/handlers_test.go | Verify OIDC callback verifies ID tokens |
| REQ01_N_020 | Test_REQ01_N_020_OIDCCallbackProviderConfigError | Negative | handlers/handlers_test.go | Verify OIDC callback handles provider config errors |
| REQ01_N_021 | Test_REQ01_N_021_OIDCCallbackInvalidClaims | Negative | handlers/handlers_test.go | Verify OIDC callback handles invalid claims |
| Auth_N_001 | Test_Auth_N_001_ConfigureJWTSecretProductionMissing | Negative | auth/auth_test.go | Verify JWT secret is required outside development |
| Auth_N_002 | Test_Auth_N_002_OIDCConfigOAuth2ConfigUnconfigured | Negative | auth/auth_test.go | Verify unconfigured OIDC config returns error |
| REQ01_E_001 | Test_REQ01_E_001_TokenExpiry | Edge | auth/auth_test.go | Verify expired tokens are rejected |
| REQ01_E_002 | Test_REQ01_E_002_RegisterDBError | Edge | handlers/handlers_test.go | Verify Register handles DB error on create |
| REQ01_E_003 | Test_REQ01_E_003_LoginDBError | Edge | handlers/handlers_test.go | Verify Login handles DB error |
| REQ01_E_004 | Test_REQ01_E_004_RegisterHashPasswordError | Edge | handlers/handlers_test.go | Verify Register handles hash password error |
| REQ01_E_005 | Test_REQ01_E_005_LoginGenerateTokenError | Edge | handlers/handlers_test.go | Verify Login handles token generation error |
| REQ01_E_006 | Test_REQ01_E_006_OIDCCallbackGenerateTokenError | Edge | handlers/handlers_test.go | Verify OIDC callback handles app token generation errors |
| REQ01_E_007 | Test_REQ01_E_007_UpsertOIDCUserDBError | Edge | handlers/handlers_test.go | Verify OIDC upsert handles DB errors |
| REQ01_E_008 | Test_REQ01_E_008_AttachOIDCIdentityDBError | Edge | handlers/handlers_test.go | Verify OIDC attach handles DB errors |

### REQ02: Create TODOs

| Test ID | Test Name | Type | File | Description |
|---------|-----------|------|------|-------------|
| REQ02_P_001 | Test_REQ02_P_001_CreateTodoSuccess | Positive | handlers/handlers_test.go | Verify authenticated user can create a todo |
| REQ02_N_001 | Test_REQ02_N_001_CreateTodoUnauthenticated | Negative | handlers/handlers_test.go | Verify unauthenticated user cannot create todo |
| REQ02_N_002 | Test_REQ02_N_002_CreateTodoEmptyTitle | Negative | handlers/handlers_test.go | Verify todo creation fails without title |
| REQ02_N_003 | Test_REQ02_N_003_CreateTodoInvalidJSON | Negative | handlers/handlers_test.go | Verify create todo fails with invalid JSON |
| REQ02_E_001 | Test_REQ02_E_001_CreateTodoDBError | Edge | handlers/handlers_test.go | Verify CreateTodo handles DB error |
| REQ02_E_002 | Test_REQ02_E_002_CreateTodoNoUserInContext | Edge | handlers/handlers_test.go | Verify CreateTodo handles missing user in context |

### REQ03: View TODOs

| Test ID | Test Name | Type | File | Description |
|---------|-----------|------|------|-------------|
| REQ03_P_001 | Test_REQ03_P_001_ListAllTodos | Positive | handlers/handlers_test.go | Verify all todos are returned |
| REQ03_P_002 | Test_REQ03_P_002_GetTodoById | Positive | handlers/handlers_test.go | Verify single todo can be retrieved |
| REQ03_P_003 | Test_REQ03_P_003_ListTodosUnauthenticated | Positive | handlers/handlers_test.go | Verify unauthenticated users can view todos |
| REQ03_N_001 | Test_REQ03_N_001_GetNonexistentTodo | Negative | handlers/handlers_test.go | Verify 404 for non-existent todo |
| REQ03_N_002 | Test_REQ03_N_002_GetTodoInvalidID | Negative | handlers/handlers_test.go | Verify invalid todo ID returns bad request |
| REQ03_E_001 | Test_REQ03_E_001_ListEmptyTodos | Edge | handlers/handlers_test.go | Verify empty list returned when no todos |
| REQ03_E_002 | Test_REQ03_E_002_ListTodosDBError | Edge | handlers/handlers_test.go | Verify ListTodos handles DB error |

### REQ04: Ownership Restrictions

| Test ID | Test Name | Type | File | Description |
|---------|-----------|------|------|-------------|
| REQ04_P_001 | Test_REQ04_P_001_UpdateOwnTodo | Positive | handlers/handlers_test.go | Verify user can update their own todo |
| REQ04_P_002 | Test_REQ04_P_002_DeleteOwnTodo | Positive | handlers/handlers_test.go | Verify user can delete their own todo |
| REQ04_P_003 | Test_REQ04_P_003_UpdateTodoCompleted | Positive | handlers/handlers_test.go | Verify updating todo completed status |
| REQ04_P_004 | Test_REQ04_P_004_UpdateTodoDescription | Positive | handlers/handlers_test.go | Verify updating todo description |
| REQ04_N_001 | Test_REQ04_N_001_UpdateOtherUserTodo | Negative | handlers/handlers_test.go | Verify user cannot update another user's todo |
| REQ04_N_002 | Test_REQ04_N_002_DeleteOtherUserTodo | Negative | handlers/handlers_test.go | Verify user cannot delete another user's todo |
| REQ04_N_003 | Test_REQ04_N_003_UpdateTodoUnauthenticated | Negative | handlers/handlers_test.go | Verify unauthenticated user cannot update |
| REQ04_N_004 | Test_REQ04_N_004_DeleteTodoUnauthenticated | Negative | handlers/handlers_test.go | Verify unauthenticated user cannot delete |
| REQ04_N_005 | Test_REQ04_N_005_UpdateTodoInvalidID | Negative | handlers/handlers_test.go | Verify invalid todo ID returns bad request |
| REQ04_N_006 | Test_REQ04_N_006_UpdateNonexistentTodo | Negative | handlers/handlers_test.go | Verify updating non-existent todo returns not found |
| REQ04_N_007 | Test_REQ04_N_007_DeleteTodoInvalidID | Negative | handlers/handlers_test.go | Verify invalid todo ID returns bad request |
| REQ04_N_008 | Test_REQ04_N_008_DeleteNonexistentTodo | Negative | handlers/handlers_test.go | Verify deleting non-existent todo returns not found |
| REQ04_N_009 | Test_REQ04_N_009_UpdateTodoInvalidJSON | Negative | handlers/handlers_test.go | Verify update todo fails with invalid JSON |
| REQ04_E_001 | Test_REQ04_E_001_UpdateTodoDBErrorOnSave | Edge | handlers/handlers_test.go | Verify UpdateTodo handles DB error on save |
| REQ04_E_002 | Test_REQ04_E_002_DeleteTodoDBErrorOnDelete | Edge | handlers/handlers_test.go | Verify DeleteTodo handles DB error on delete |
| REQ04_E_003 | Test_REQ04_E_003_UpdateTodoNoUserInContext | Edge | handlers/handlers_test.go | Verify UpdateTodo handles missing user in context |
| REQ04_E_004 | Test_REQ04_E_004_DeleteTodoNoUserInContext | Edge | handlers/handlers_test.go | Verify DeleteTodo handles missing user in context |

### Infrastructure Tests

| Test ID | Test Name | Type | File | Description |
|---------|-----------|------|------|-------------|
| DB_P_001 | Test_DB_P_001_InitDBSuccess | Positive | db/db_test.go | Verify database initialization works |
| DB_P_002 | Test_DB_P_002_GetDBReturnsInstance | Positive | db/db_test.go | Verify GetDB returns the database instance |
| DB_P_003 | Test_DB_P_003_SetDB | Positive | db/db_test.go | Verify SetDB sets the database instance |
| DB_P_004 | Test_DB_P_004_ConfigFromEnv | Positive | db/db_test.go | Verify database config is read from environment |
| DB_P_005 | Test_DB_P_005_PostgresDSN | Positive | db/db_test.go | Verify PostgreSQL DSN construction |
| DB_P_006 | Test_DB_P_006_BuildRDSAuthToken | Positive | db/db_test.go | Verify RDS IAM auth token signing |
| DB_P_007 | Test_DB_P_007_PostgresOpeners | Positive | db/db_test.go | Verify PostgreSQL openers initialize connections |
| DB_P_008 | Test_DB_P_008_SmallHelpers | Positive | db/db_test.go | Verify database helper defaults |
| DB_P_009 | Test_DB_P_009_OpenIAMPostgres | Positive | db/db_test.go | Verify IAM PostgreSQL sql.DB initialization |
| DB_N_001 | Test_DB_N_001_InitDBInvalidPath | Negative | db/db_test.go | Verify database initialization fails with invalid path |
| DB_N_002 | Test_DB_N_002_InitDBAutoMigrateError | Negative | db/db_test.go | Verify InitDB handles AutoMigrate error |
| DB_N_003 | Test_DB_N_003_OpenDBUnsupportedDriver | Negative | db/db_test.go | Verify unsupported drivers fail fast |
| DB_N_004 | Test_DB_N_004_ValidatePostgresConfig | Negative | db/db_test.go | Verify PostgreSQL config validation |
| DB_N_005 | Test_DB_N_005_IAMAuthConnectorConnectError | Negative | db/db_test.go | Verify IAM connector connection errors |
| DB_N_006 | Test_DB_N_006_BuildRDSAuthTokenCredentialError | Negative | db/db_test.go | Verify RDS IAM auth token credential errors |
| Models_P_001 | Test_Models_P_001_UserTableName | Positive | models/models_test.go | Verify User.TableName returns correct table name |
| Models_P_002 | Test_Models_P_002_TodoTableName | Positive | models/models_test.go | Verify Todo.TableName returns correct table name |
| Main_P_001 | Test_Main_P_001_SetupRouter | Positive | main_test.go | Verify router setup works correctly |
| Main_P_002 | Test_Main_P_002_HealthCheck | Positive | main_test.go | Verify health check endpoint works |
| Main_P_002B | Test_Main_P_002B_AuthConfig | Positive | main_test.go | Verify auth config endpoint exposes OIDC state |
| Main_P_003 | Test_Main_P_003_CORSMiddlewareDefaultSameOrigin | Positive | main_test.go | Verify CORS defaults to same-origin only |
| Main_P_003B | Test_Main_P_003B_CORSMiddlewareConfigured | Positive | main_test.go | Verify configured CORS origin is allowed |
| Main_P_004 | Test_Main_P_004_CORSMiddlewareOptions | Positive | main_test.go | Verify OPTIONS request handling |
| Main_P_005 | Test_Main_P_005_GetDBPathDefault | Positive | main_test.go | Verify default DB path |
| Main_P_006 | Test_Main_P_006_GetDBPathEnv | Positive | main_test.go | Verify DB path from environment |
| Main_P_007 | Test_Main_P_007_GetPortDefault | Positive | main_test.go | Verify default port |
| Main_P_008 | Test_Main_P_008_GetPortEnv | Positive | main_test.go | Verify port from environment |
| Main_P_009 | Test_Main_P_009_RunSuccess | Positive | main_test.go | Verify startup orchestration succeeds |
| Main_N_001 | Test_Main_N_001_RunAuthConfigError | Negative | main_test.go | Verify startup fails on auth config errors |
| Main_N_002 | Test_Main_N_002_RunDBError | Negative | main_test.go | Verify startup fails on database errors |
| Main_N_003 | Test_Main_N_003_RunServerError | Negative | main_test.go | Verify startup fails on server errors |

## 4. Coverage Summary

| Requirement | Total Tests | Positive | Negative | Edge |
|-------------|-------------|----------|----------|------|
| REQ01 | 50 | 19 | 23 | 8 |
| REQ02 | 6 | 1 | 3 | 2 |
| REQ03 | 7 | 3 | 2 | 2 |
| REQ04 | 17 | 4 | 9 | 4 |
| Infrastructure | 31 | 22 | 9 | 0 |
| **Total** | **111** | **49** | **46** | **16** |

## 5. Code Coverage

| Package | Coverage |
|---------|----------|
| auth | 100.0% |
| db | 100.0% |
| handlers | 100.0% |
| models | 100.0% |
| **Total** | **100.0%** |

*Note: main.go is excluded from coverage as it contains only the application entry point.*

## 6. Test Execution

Tests are executed using:
```bash
go test -v ./...
```

Coverage report (excluding main.go):
```bash
go test -coverprofile=coverage.out ./auth ./db ./handlers ./models
go tool cover -func=coverage.out
```

HTML coverage report:
```bash
go tool cover -html=coverage.out -o coverage.html
```

## 7. Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2025-12-02 | Container Solutions | Initial traceability matrix |
| 1.1.0 | 2025-12-03 | Container Solutions | Updated for 100% code coverage, added infrastructure tests, expanded test matrix |
