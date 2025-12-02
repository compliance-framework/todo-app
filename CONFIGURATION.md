# Leo App - Configuration Documentation

## 1. Overview

This document identifies all software configuration items (SCIs) for the Leo App application as required by DO-178C configuration management practices.

## 2. Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | HTTP server port | `8080` | No |
| `DB_PATH` | Path to SQLite database file | `leo_app.db` | No |

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
| Secret Key | Configurable | `auth/auth.go` |

**Note**: In production, the JWT secret should be provided via environment variable.

### 5.2 Password Hashing
| Setting | Value |
|---------|-------|
| Algorithm | bcrypt |
| Cost | Default (10) |

## 6. API Configuration

### 6.1 CORS Settings
| Setting | Value |
|---------|-------|
| Allow Origin | `*` |
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
