package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ContainerSolutions/todo-app/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// AutoMigrateFunc is the function used for auto-migration (can be mocked for testing)
var AutoMigrateFunc = func(db *gorm.DB) error {
	return db.AutoMigrate(&models.User{}, &models.Todo{})
}

// Config contains database connection settings.
type Config struct {
	Driver      string
	SQLitePath  string
	Host        string
	Port        string
	Name        string
	User        string
	Password    string
	Region      string
	SSLMode     string
	SSLRootCert string
	IAMAuth     bool
}

// ConfigFromEnv builds database config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Driver:      envOrDefault("DB_DRIVER", "sqlite"),
		SQLitePath:  envOrDefault("DB_PATH", "todo_app.db"),
		Host:        os.Getenv("DB_HOST"),
		Port:        envOrDefault("DB_PORT", "5432"),
		Name:        os.Getenv("DB_NAME"),
		User:        os.Getenv("DB_USER"),
		Password:    os.Getenv("DB_PASSWORD"),
		Region:      firstNonEmpty(os.Getenv("DB_REGION"), os.Getenv("AWS_REGION")),
		SSLMode:     envOrDefault("DB_SSLMODE", "verify-full"),
		SSLRootCert: firstNonEmpty(os.Getenv("DB_SSLROOTCERT"), os.Getenv("DB_RDS_CA_CERT_PATH"), defaultRDSCABundlePath()),
		IAMAuth:     envBoolOrDefault("DB_IAM_AUTH", true),
	}
}

// InitDB initializes the database connection and runs migrations
func InitDB(dbPath string) error {
	return InitDBWithConfig(context.Background(), Config{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	})
}

// InitDBWithConfig initializes the configured database connection and runs migrations.
func InitDBWithConfig(ctx context.Context, cfg Config) error {
	database, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}

	// Auto-migrate the schema
	if err := AutoMigrateFunc(database); err != nil {
		return err
	}

	DB = database
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

func openDB(ctx context.Context, cfg Config) (*gorm.DB, error) {
	switch strings.ToLower(cfg.Driver) {
	case "", "sqlite":
		return gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{})
	case "postgres", "postgresql":
		return openPostgres(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.Driver)
	}
}

func openPostgres(ctx context.Context, cfg Config) (*gorm.DB, error) {
	if err := validatePostgresConfig(cfg); err != nil {
		return nil, err
	}

	if cfg.IAMAuth {
		sqlDB, err := openIAMPostgres(ctx, cfg)
		if err != nil {
			return nil, err
		}
		return gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	}

	return gorm.Open(postgres.Open(postgresDSN(cfg, cfg.Password)), &gorm.Config{})
}

func validatePostgresConfig(cfg Config) error {
	if cfg.Host == "" || cfg.Name == "" || cfg.User == "" {
		return errors.New("DB_HOST, DB_NAME, and DB_USER are required for PostgreSQL")
	}
	if cfg.IAMAuth && cfg.Region == "" {
		return errors.New("DB_REGION or AWS_REGION is required when DB_IAM_AUTH is enabled")
	}
	if strings.EqualFold(cfg.SSLMode, "disable") {
		return errors.New("PostgreSQL connections must use TLS; DB_SSLMODE cannot be disable")
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return fmt.Errorf("invalid DB_PORT %q: %w", cfg.Port, err)
	}
	return nil
}

func openIAMPostgres(ctx context.Context, cfg Config) (*sql.DB, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	sqlDB := sql.OpenDB(&iamAuthConnector{
		config:      cfg,
		credentials: awsCfg.Credentials,
	})
	sqlDB.SetConnMaxLifetime(14 * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Minute)

	return sqlDB, nil
}

type iamAuthConnector struct {
	config      Config
	credentials aws.CredentialsProvider
}

func (c *iamAuthConnector) Connect(ctx context.Context) (driver.Conn, error) {
	token, err := buildRDSAuthToken(ctx, c.config, c.credentials)
	if err != nil {
		return nil, err
	}

	connConfig, err := pgx.ParseConfig(postgresDSN(c.config, token))
	if err != nil {
		return nil, err
	}

	return stdlib.GetConnector(*connConfig).Connect(ctx)
}

func (c *iamAuthConnector) Driver() driver.Driver {
	return stdlib.GetDefaultDriver()
}

func buildRDSAuthToken(ctx context.Context, cfg Config, credentials aws.CredentialsProvider) (string, error) {
	creds, err := credentials.Retrieve(ctx)
	if err != nil {
		return "", fmt.Errorf("retrieve AWS credentials: %w", err)
	}

	endpoint := net.JoinHostPort(cfg.Host, cfg.Port)
	authURL := url.URL{
		Scheme: "https",
		Host:   endpoint,
		Path:   "/",
	}
	query := authURL.Query()
	query.Set("Action", "connect")
	query.Set("DBUser", cfg.User)
	query.Set("X-Amz-Expires", "900")
	authURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL.String(), nil)
	if err != nil {
		return "", err
	}

	signedURL, _, err := v4.NewSigner().PresignHTTP(
		ctx,
		creds,
		req,
		"UNSIGNED-PAYLOAD",
		"rds-db",
		cfg.Region,
		time.Now(),
		func(options *v4.SignerOptions) {
			options.DisableURIPathEscaping = true
		},
	)
	if err != nil {
		return "", fmt.Errorf("sign RDS IAM auth token: %w", err)
	}

	return strings.TrimPrefix(signedURL, "https://"), nil
}

func postgresDSN(cfg Config, password string) string {
	dsn := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   cfg.Name,
	}
	if password == "" {
		dsn.User = url.User(cfg.User)
	} else {
		dsn.User = url.UserPassword(cfg.User, password)
	}

	query := dsn.Query()
	query.Set("sslmode", cfg.SSLMode)
	if cfg.SSLRootCert != "" {
		query.Set("sslrootcert", cfg.SSLRootCert)
	}
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return enabled
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultRDSCABundlePath() string {
	for _, path := range []string{
		"/etc/ssl/certs/global-bundle.pem",
		"/opt/rds-ca/global-bundle.pem",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
