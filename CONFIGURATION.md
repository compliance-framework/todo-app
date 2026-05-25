# Leo App - Configuration Documentation

## 1. Overview

This document identifies all software configuration items (SCIs) for the Leo App application as required by DO-178C configuration management practices.

## 2. Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | HTTP server port | `8080` | No |
| `APP_ENV` / `ENV` / `GIN_MODE` | Runtime mode used to enforce production secrets | none | No |
| `JWT_SECRET` | JWT signing secret | none | Yes outside development |
| `CORS_ALLOWED_ORIGIN` | Allowed cross-origin browser origin; empty means same-origin only | none | No |
| `DB_DRIVER` | Database driver: `sqlite` or `postgres` | `sqlite` | No |
| `DB_PATH` | Path to SQLite database file | `todo_app.db` | No |
| `DB_HOST` | PostgreSQL/RDS hostname | none | Yes for PostgreSQL |
| `DB_PORT` | PostgreSQL/RDS port | `5432` | No |
| `DB_NAME` | PostgreSQL database name | none | Yes for PostgreSQL |
| `DB_USER` | PostgreSQL database user | none | Yes for PostgreSQL |
| `DB_REGION` / `AWS_REGION` | AWS region for RDS IAM authentication | none | Yes when `DB_IAM_AUTH=true` |
| `DB_SSLMODE` | PostgreSQL TLS mode; supported values: `verify-full`, `verify-ca` | `verify-full` | No |
| `DB_SSLROOTCERT` / `DB_RDS_CA_CERT_PATH` | AWS RDS CA bundle path | auto-detected common path | No |
| `DB_IAM_AUTH` | Generate RDS IAM auth token from AWS default credentials | `true` | No |
| `DB_PASSWORD` | Optional PostgreSQL password for non-IAM local connections | none | No |
| `DB_MAX_OPEN_CONNS` | PostgreSQL max open connections | `25` | No |
| `DB_MAX_IDLE_CONNS` | PostgreSQL max idle connections | `5` | No |
| `OIDC_ISSUER_URL` | OIDC issuer URL | none | Required for OIDC |
| `OIDC_CLIENT_ID` | OIDC client ID | none | Required for OIDC |
| `OIDC_CLIENT_SECRET` | OIDC client secret | none | Required for OIDC |
| `OIDC_REDIRECT_URL` | OIDC redirect URL | none | Required for OIDC |
| `OIDC_STATE_SECRET` | OIDC state cookie signing secret; falls back to `JWT_SECRET` when unset | `JWT_SECRET` | No |
| `OIDC_COOKIE_SECURE` | Whether OIDC state cookies use the `Secure` attribute | `true` | No |
| `OIDC_CODE_VERIFIER_STORE_MAX_ENTRIES` | Maximum in-memory OIDC PKCE verifier entries | `1024` | No |

## 3. Source Code Configuration Items

### 3.1 Go Module Configuration
- **File**: `go.mod`
- **Purpose**: Defines module path and dependencies
- **Version Control**: Yes

### 3.2 Linting Configuration
- **File**: `.golangci.yml`
- **Purpose**: Defines golangci-lint rules and settings
- **Version Control**: Yes

## 4. Database Schema

### 4.1 Users Table
| Column | Type | Constraints |
|--------|------|-------------|
| id | INTEGER | PRIMARY KEY, AUTO INCREMENT |
| created_at | DATETIME | NOT NULL |
| updated_at | DATETIME | NOT NULL |
| deleted_at | DATETIME | INDEX (soft delete) |
| username | VARCHAR(255) | UNIQUE, NOT NULL |
| password | VARCHAR(255) | NOT NULL (bcrypt hash) |
| email | VARCHAR(320) | UNIQUE, nullable |
| oidc_issuer | VARCHAR(512) | Composite unique index with oidc_subject, nullable |
| oidc_subject | VARCHAR(255) | Composite unique index with oidc_issuer, nullable |
| auth_provider | VARCHAR(32) | NOT NULL, default password |

### 4.2 Todos Table
| Column | Type | Constraints |
|--------|------|-------------|
| id | INTEGER | PRIMARY KEY, AUTO INCREMENT |
| created_at | DATETIME | NOT NULL |
| updated_at | DATETIME | NOT NULL |
| deleted_at | DATETIME | INDEX (soft delete) |
| title | VARCHAR(255) | NOT NULL |
| description | VARCHAR(1000) | |
| completed | BOOLEAN | DEFAULT FALSE |
| user_id | INTEGER | NOT NULL, FOREIGN KEY (users.id) |

## 5. Authentication Configuration

### 5.1 JWT Settings
| Setting | Value | Location |
|---------|-------|----------|
| Signing Method | HS256 | `auth/auth.go` |
| Token Expiry | 24 hours | `auth/auth.go` |
| Secret Key | `JWT_SECRET`; development fallback only | `auth/auth.go` |

**Note**: In production, `JWT_SECRET` must be provided via environment variable. The runtime mode has no implicit development default for secret enforcement; set `APP_ENV`, `ENV`, or `GIN_MODE` explicitly to `debug`, `dev`, `development`, `local`, or `test` to use the development JWT secret fallback. If all runtime mode variables are unset, `JWT_SECRET` is required.

### 5.2 Password Hashing
| Setting | Value |
|---------|-------|
| Algorithm | bcrypt |
| Cost | Default (10) |

### 5.3 OIDC Settings
| Setting | Value |
|---------|-------|
| Flow | Authorization code |
| Provider config | `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` |
| State cookie signing | `OIDC_STATE_SECRET`, falling back to `JWT_SECRET` |
| User mapping | Existing `User` model keyed by OIDC issuer/subject and email |

## 6. API Configuration

### 6.1 CORS Settings
| Setting | Value |
|---------|-------|
| Allow Origin | `CORS_ALLOWED_ORIGIN`; empty means same-origin only |
| Allow Methods | GET, POST, PUT, DELETE, OPTIONS |
| Allow Headers | Content-Type, Authorization |

### 6.2 Request Validation
| Field | Constraint |
|-------|------------|
| Username | min=3, max=255 |
| Password | min=6 |
| Todo Title | min=1, max=255 |
| Todo Description | max=1000 |

## 7. CI/CD Configuration

### 7.1 GitHub Actions Workflow
- **File**: `.github/workflows/ci.yml`
- **Triggers**: Push to main, Pull requests
- **Jobs**: Test, Lint, Coverage

### 7.2 Branch Protection
| Setting | Value |
|---------|-------|
| Protected Branch | main |
| Required Reviews | 1 |
| Required Status Checks | test, lint |
| Dismiss Stale Reviews | Yes |

## 8. Configuration Change Log

| Version | Date | Change |
|---------|------|--------|
| 1.0.0 | 2025-12-02 | Initial configuration |
