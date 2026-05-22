package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// Test_DB_P_004_ConfigFromEnv verifies database config is read from environment.
func Test_DB_P_004_ConfigFromEnv(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_NAME", "todo")
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_REGION", "")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("DB_SSLROOTCERT", "/tmp/rds.pem")
	t.Setenv("DB_IAM_AUTH", "false")
	t.Setenv("DB_MAX_OPEN_CONNS", "12")
	t.Setenv("DB_MAX_IDLE_CONNS", "4")

	cfg := ConfigFromEnv()
	if cfg.Driver != "postgres" || cfg.SQLitePath != "/tmp/test.db" || cfg.Host != "db.example.com" {
		t.Fatalf("Unexpected config: %+v", cfg)
	}
	if cfg.Port != "6543" || cfg.Name != "todo" || cfg.User != "app" || cfg.Password != "password" {
		t.Fatalf("Unexpected PostgreSQL config: %+v", cfg)
	}
	if cfg.Region != "us-east-1" || cfg.SSLMode != "verify-full" || cfg.SSLRootCert != "/tmp/rds.pem" || cfg.IAMAuth {
		t.Fatalf("Unexpected AWS/TLS config: %+v", cfg)
	}
	if cfg.MaxOpenConns != 12 || cfg.MaxIdleConns != 4 {
		t.Fatalf("Unexpected pool config: %+v", cfg)
	}
}

// Test_DB_N_003_OpenDBUnsupportedDriver verifies unsupported drivers fail fast.
func Test_DB_N_003_OpenDBUnsupportedDriver(t *testing.T) {
	_, err := openDB(t.Context(), Config{Driver: "mysql"})
	if err == nil {
		t.Error("Expected unsupported driver error")
	}
}

// Test_DB_N_004_ValidatePostgresConfig verifies PostgreSQL config validation.
func Test_DB_N_004_ValidatePostgresConfig(t *testing.T) {
	valid := Config{
		Host:    "db.example.com",
		Port:    "5432",
		Name:    "todo",
		User:    "app",
		Region:  "us-east-1",
		SSLMode: "verify-full",
		IAMAuth: true,
	}

	if err := validatePostgresConfig(valid); err != nil {
		t.Fatalf("Expected valid config, got %v", err)
	}
	if _, err := openPostgres(t.Context(), Config{Driver: "postgres"}); err == nil {
		t.Error("Expected openPostgres validation error")
	}

	for name, cfg := range map[string]Config{
		"missing required":  {Port: "5432", Region: "us-east-1", SSLMode: "verify-full", IAMAuth: true},
		"missing region":    {Host: "db.example.com", Port: "5432", Name: "todo", User: "app", SSLMode: "verify-full", IAMAuth: true},
		"disabled tls":      {Host: "db.example.com", Port: "5432", Name: "todo", User: "app", Region: "us-east-1", SSLMode: "disable", IAMAuth: true},
		"invalid port":      {Host: "db.example.com", Port: "bad", Name: "todo", User: "app", Region: "us-east-1", SSLMode: "verify-full", IAMAuth: true},
		"negative max open": {Host: "db.example.com", Port: "5432", Name: "todo", User: "app", Region: "us-east-1", SSLMode: "verify-full", IAMAuth: true, MaxOpenConns: -1},
		"negative max idle": {Host: "db.example.com", Port: "5432", Name: "todo", User: "app", Region: "us-east-1", SSLMode: "verify-full", IAMAuth: true, MaxIdleConns: -1},
	} {
		if err := validatePostgresConfig(cfg); err == nil {
			t.Errorf("Expected validation error for %s", name)
		}
	}
}

// Test_DB_P_005_PostgresDSN verifies DSN construction for PostgreSQL.
func Test_DB_P_005_PostgresDSN(t *testing.T) {
	cfg := Config{
		Host:        "db.example.com",
		Port:        "5432",
		Name:        "todo",
		User:        "app",
		SSLMode:     "verify-full",
		SSLRootCert: "/tmp/rds-ca.pem",
	}

	dsn := postgresDSN(cfg, "secret")
	for _, part := range []string{
		"postgres://app:secret@db.example.com:5432/todo",
		"sslmode=verify-full",
		"sslrootcert=%2Ftmp%2Frds-ca.pem",
	} {
		if !strings.Contains(dsn, part) {
			t.Errorf("Expected DSN to contain %q, got %q", part, dsn)
		}
	}

	dsn = postgresDSN(cfg, "")
	if !strings.Contains(dsn, "postgres://app@db.example.com:5432/todo") {
		t.Errorf("Expected passwordless DSN, got %q", dsn)
	}
}

// Test_DB_P_006_BuildRDSAuthToken verifies IAM auth token signing.
func Test_DB_P_006_BuildRDSAuthToken(t *testing.T) {
	cfg := Config{
		Host:   "db.example.com",
		Port:   "5432",
		User:   "app",
		Region: "us-east-1",
	}
	provider := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "AKIDEXAMPLE",
			SecretAccessKey: "secret",
			SessionToken:    "session",
		}, nil
	})

	token, err := buildRDSAuthToken(t.Context(), cfg, provider)
	if err != nil {
		t.Fatalf("buildRDSAuthToken returned error: %v", err)
	}
	for _, part := range []string{
		"db.example.com:5432/",
		"Action=connect",
		"DBUser=app",
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Expires=900",
	} {
		if !strings.Contains(token, part) {
			t.Errorf("Expected token to contain %q, got %q", part, token)
		}
	}
}

// Test_DB_N_006_BuildRDSAuthTokenCredentialError verifies credential errors are returned.
func Test_DB_N_006_BuildRDSAuthTokenCredentialError(t *testing.T) {
	provider := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, errors.New("no credentials")
	})
	_, err := buildRDSAuthToken(t.Context(), Config{}, provider)
	if err == nil {
		t.Error("Expected credential error")
	}
}

// Test_DB_N_005_IAMAuthConnectorConnectError verifies IAM connector connection errors.
func Test_DB_N_005_IAMAuthConnectorConnectError(t *testing.T) {
	cfg := Config{
		Host:    "127.0.0.1",
		Port:    "1",
		Name:    "todo",
		User:    "app",
		Region:  "us-east-1",
		SSLMode: "verify-full",
	}
	provider := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "AKIDEXAMPLE",
			SecretAccessKey: "secret",
		}, nil
	})
	connector := &iamAuthConnector{config: cfg, credentials: provider}

	if connector.Driver() == nil {
		t.Error("Expected connector driver")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, err := connector.Connect(ctx); err == nil {
		t.Error("Expected connection error")
	}

	connector.credentials = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, errors.New("no credentials")
	})
	if _, err := connector.Connect(t.Context()); err == nil {
		t.Error("Expected credential error")
	}
}

// Test_DB_P_009_OpenIAMPostgres verifies IAM PostgreSQL sql.DB initialization.
func Test_DB_P_009_OpenIAMPostgres(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	sqlDB, err := openIAMPostgres(t.Context(), Config{
		Host:         "db.example.com",
		Port:         "5432",
		Name:         "todo",
		User:         "app",
		Region:       "us-east-1",
		SSLMode:      "verify-full",
		MaxOpenConns: 9,
		MaxIdleConns: 2,
	})
	if err != nil {
		t.Fatalf("openIAMPostgres returned error: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 9 {
		t.Fatalf("Expected MaxOpenConnections 9, got %d", got)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

// Test_DB_P_010_PostgresPoolDefaults verifies bounded pool defaults are applied.
func Test_DB_P_010_PostgresPoolDefaults(t *testing.T) {
	database, err := openPostgres(t.Context(), Config{
		Driver:   "postgres",
		Host:     "db.example.com",
		Port:     "5432",
		Name:     "todo",
		User:     "app",
		Password: "password",
		Region:   "us-east-1",
		SSLMode:  "verify-full",
		IAMAuth:  false,
	})
	if err != nil {
		t.Fatalf("openPostgres returned error: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database.DB returned error: %v", err)
	}
	defer sqlDB.Close()

	if got := sqlDB.Stats().MaxOpenConnections; got != defaultPostgresMaxOpenConns {
		t.Fatalf("Expected MaxOpenConnections %d, got %d", defaultPostgresMaxOpenConns, got)
	}
}

// Test_DB_P_011_PostgresPoolConfigured verifies configured pool sizes are applied.
func Test_DB_P_011_PostgresPoolConfigured(t *testing.T) {
	database, err := openPostgres(t.Context(), Config{
		Driver:       "postgres",
		Host:         "db.example.com",
		Port:         "5432",
		Name:         "todo",
		User:         "app",
		Password:     "password",
		Region:       "us-east-1",
		SSLMode:      "verify-full",
		IAMAuth:      false,
		MaxOpenConns: 7,
		MaxIdleConns: 3,
	})
	if err != nil {
		t.Fatalf("openPostgres returned error: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database.DB returned error: %v", err)
	}
	defer sqlDB.Close()

	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("Expected MaxOpenConnections 7, got %d", got)
	}
}

// Test_DB_P_007_PostgresOpeners verifies PostgreSQL openers initialize connections.
func Test_DB_P_007_PostgresOpeners(t *testing.T) {
	cfg := Config{
		Driver:   "postgres",
		Host:     "db.example.com",
		Port:     "5432",
		Name:     "todo",
		User:     "app",
		Password: "password",
		Region:   "us-east-1",
		SSLMode:  "verify-full",
		IAMAuth:  false,
	}

	database, err := openPostgres(t.Context(), cfg)
	if err != nil {
		t.Fatalf("openPostgres returned error: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database.DB returned error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	database, err = openDB(t.Context(), cfg)
	if err != nil {
		t.Fatalf("openDB returned error: %v", err)
	}
	sqlDB, err = database.DB()
	if err != nil {
		t.Fatalf("database.DB returned error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	cfg.IAMAuth = true
	database, err = openPostgres(t.Context(), cfg)
	if err != nil {
		t.Fatalf("IAM openPostgres returned error: %v", err)
	}
	sqlDB, err = database.DB()
	if err != nil {
		t.Fatalf("database.DB returned error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

// Test_DB_P_008_SmallHelpers verifies database helper defaults.
func Test_DB_P_008_SmallHelpers(t *testing.T) {
	t.Setenv("TEST_ENV_OR_DEFAULT", "")
	if got := envOrDefault("TEST_ENV_OR_DEFAULT", "fallback"); got != "fallback" {
		t.Errorf("Expected fallback, got %q", got)
	}
	t.Setenv("TEST_ENV_OR_DEFAULT", "value")
	if got := envOrDefault("TEST_ENV_OR_DEFAULT", "fallback"); got != "value" {
		t.Errorf("Expected value, got %q", got)
	}

	t.Setenv("TEST_BOOL", "not-bool")
	if !envBoolOrDefault("TEST_BOOL", true) {
		t.Error("Expected invalid bool to return fallback")
	}
	t.Setenv("TEST_BOOL", "false")
	if envBoolOrDefault("TEST_BOOL", true) {
		t.Error("Expected false bool value")
	}
	t.Setenv("TEST_BOOL", "")
	if !envBoolOrDefault("TEST_BOOL", true) {
		t.Error("Expected empty bool to return fallback")
	}

	if firstNonEmpty("", "first", "second") != "first" {
		t.Error("Expected first non-empty value")
	}
	if firstNonEmpty("", "") != "" {
		t.Error("Expected empty result when all values are empty")
	}

	caPath := filepath.Join(t.TempDir(), "global-bundle.pem")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	originalPaths := rdsCABundlePaths
	rdsCABundlePaths = []string{filepath.Join(t.TempDir(), "missing.pem"), caPath}
	defer func() { rdsCABundlePaths = originalPaths }()
	if defaultRDSCABundlePath() != caPath {
		t.Error("Expected defaultRDSCABundlePath to find existing CA bundle")
	}
}
