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

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **Database**: SQLite with GORM
- **Authentication**: JWT (golang-jwt/jwt/v5)
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
- `DB_PATH` - SQLite database path (default: todo_app.db)

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
