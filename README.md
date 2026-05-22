# todo App - Todo List Application

A simple Todo List web application built with Go, demonstrating DO-178C compliance practices.

## Requirements

| ID | Requirement |
|----|-------------|
| REQ01 | Users should be able to LOGIN |
| REQ02 | Users should be able to create new TODOs |
| REQ03 | Users should be able to see all todo lists |
| REQ04 | Users should NOT be able to modify/delete TODOs they did not create |

## Tech Stack

- **Language**: Go 1.25+
- **Web Framework**: Gin
- **Database**: SQLite or PostgreSQL with GORM
- **Authentication**: JWT (golang-jwt/jwt/v5), optional OIDC login
- **Password Hashing**: bcrypt

## Project Structure

```
todo_app/
├── main.go              # Application entry point
├── go.mod               # Go module definition
├── models/
│   └── models.go        # Database models (User, Todo)
├── db/
│   └── db.go            # Database initialization
├── auth/
│   └── auth.go          # Authentication middleware and helpers
├── handlers/
│   ├── auth_handler.go  # Login/Register handlers
│   └── todo_handler.go  # Todo CRUD handlers
├── PLAN.md              # Software development plan
├── CONFIGURATION.md     # Configuration documentation
└── TEST_REQUIREMENT_TRACE.md  # Requirements traceability matrix
```

## API Endpoints

### Public Endpoints
- `POST /api/register` - Register a new user
- `POST /api/login` - Login and receive JWT token
- `GET /api/auth/config` - Return public auth configuration
- `GET /api/auth/oidc/login` - Start OIDC authorization-code login
- `GET /api/auth/oidc/callback` - Complete OIDC login and receive JWT token
- `GET /api/todos` - List all todos (REQ03)
- `GET /api/todos/:id` - Get a specific todo (REQ03)

### Protected Endpoints (require Authorization header)
- `POST /api/todos` - Create a new todo (REQ02)
- `PUT /api/todos/:id` - Update a todo (REQ04 - owner only)
- `DELETE /api/todos/:id` - Delete a todo (REQ04 - owner only)

## Running the Application

```bash
# Download dependencies
go mod tidy

# Run the application
go run main.go

# Or build and run
go build -o todo-app
./todo-app
```

## Environment Variables

- `PORT` - Server port (default: 8080)
- `APP_ENV` / `ENV` / `GIN_MODE` - Runtime mode; set production values to require `JWT_SECRET`
- `JWT_SECRET` - JWT signing secret, required outside development mode
- `CORS_ALLOWED_ORIGIN` - Allowed browser origin for CORS; empty defaults to same-origin only
- `DB_DRIVER` - Database driver: `sqlite` or `postgres` (default: `sqlite`)
- `DB_PATH` - SQLite database path (default: todo_app.db)
- `DB_HOST` - PostgreSQL/RDS host
- `DB_PORT` - PostgreSQL/RDS port (default: 5432)
- `DB_NAME` - PostgreSQL database name
- `DB_USER` - PostgreSQL database user
- `DB_REGION` / `AWS_REGION` - AWS region for RDS IAM authentication
- `DB_SSLMODE` - PostgreSQL TLS mode (default: verify-full; `disable` is rejected)
- `DB_SSLROOTCERT` / `DB_RDS_CA_CERT_PATH` - RDS CA bundle path
- `DB_IAM_AUTH` - Enable RDS IAM auth token generation (default: true)
- `DB_PASSWORD` - Optional PostgreSQL password for non-IAM connections (`DB_IAM_AUTH=false`)
- `DB_MAX_OPEN_CONNS` - PostgreSQL max open connections (default: 25)
- `DB_MAX_IDLE_CONNS` - PostgreSQL max idle connections (default: 5)
- `OIDC_ISSUER_URL` - OIDC issuer URL
- `OIDC_CLIENT_ID` - OIDC client ID
- `OIDC_CLIENT_SECRET` - OIDC client secret
- `OIDC_REDIRECT_URL` - OIDC redirect URL
- `OIDC_COOKIE_SECURE` - Set OIDC state cookie `Secure` attribute (default: true; set false only for local HTTP)

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

MIT
