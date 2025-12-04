# GoFiber API Example

## Running Releaser Commands

### Build Only
```bash
# From project root
cd examples/gofiber-api

# Build all targets
releaser build --config .releaser.yaml

# Build snapshot (no git tag required)
releaser build --config .releaser.yaml --snapshot

# Build single target
releaser build --config .releaser.yaml --single-target amd64
```

### Full Release
```bash
# Create a git tag first
git tag v1.0.0

# Run full release pipeline
releaser release --config .releaser.yaml

# Snapshot release (no publishing)
releaser release --config .releaser.yaml --snapshot

# Skip publishing
releaser release --config .releaser.yaml --skip-publish
```

### Using Minimal Config
```bash
# Use the minimal configuration
releaser build --config releaser.minimal.yaml
releaser release --config releaser.minimal.yaml
```

### Docker Export
```bash
# Builds now automatically export Docker images as tar artifacts
releaser build --config .releaser.yaml

# Images will be exported to:
# - dist/gofiber-api_<version>_docker.tar.gz
# - dist/gofiber-api_<version>_docker.tar
```

## Environment Variables

Set these before running:
```bash
export GITHUB_OWNER=your-username
export GITHUB_REPO=gofiber-api
export GITHUB_TOKEN=your-token
```

Or run with inline variables:
```bash
GITHUB_OWNER=myusername GITHUB_REPO=gofiber-api releaser build --config .releaser.yaml --snapshot
```

## Features

- **REST API** with Users, Tasks, and Products endpoints
- **In-memory storage** with sample data
- **Graceful shutdown** handling
- **CORS** support
- **Request logging** and recovery middleware
- **Health check** endpoint
- **Docker** support with multi-stage build
- **Systemd** service file for Linux deployment

## API Endpoints

### Health Check
- `GET /health` - Health check endpoint

### Users
- `GET /api/v1/users` - List all users (paginated)
- `GET /api/v1/users/:id` - Get a user by ID
- `POST /api/v1/users` - Create a new user
- `PUT /api/v1/users/:id` - Update a user
- `DELETE /api/v1/users/:id` - Delete a user

### Tasks
- `GET /api/v1/tasks` - List all tasks (paginated)
- `GET /api/v1/tasks/:id` - Get a task by ID
- `POST /api/v1/tasks` - Create a new task
- `PUT /api/v1/tasks/:id` - Update a task
- `DELETE /api/v1/tasks/:id` - Delete a task

### Products
- `GET /api/v1/products` - List all products (paginated)
- `GET /api/v1/products/:id` - Get a product by ID
- `POST /api/v1/products` - Create a new product
- `PUT /api/v1/products/:id` - Update a product
- `DELETE /api/v1/products/:id` - Delete a product

## Quick Start

### Run locally

```bash
# Install dependencies
go mod tidy

# Run the server
go run ./cmd/server

# Or with a config file
go run ./cmd/server --config config.example.yaml
```

### Build with Releaser

```bash
# Snapshot build (no git tag required)
releaser build --snapshot

# Full release (requires git tag)
git tag v1.0.0
releaser release
```

### Run with Docker

```bash
# Build the image
docker build -t gofiber-api:latest .

# Run the container
docker run -p 3000:3000 gofiber-api:latest
```

## Testing the API

```bash
# Health check
curl http://localhost:3000/health

# List users
curl http://localhost:3000/api/v1/users

# Create a user
curl -X POST http://localhost:3000/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Test User", "email": "test@example.com"}'

# Create a task
curl -X POST http://localhost:3000/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"title": "My Task", "description": "Task description", "priority": 2}'

# Create a product
curl -X POST http://localhost:3000/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{"sku": "TEST-001", "name": "Test Product", "price": 29.99, "currency": "USD", "category": "Test"}'
```

## Configuration

Configuration can be provided via:

1. Command line flag: `--config /path/to/config.yaml`
2. Current directory: `./config.yaml`
3. System config: `/etc/gofiber-api/config.yaml`
4. User config: `~/.config/gofiber-api/config.yaml`
5. Environment variables:
   - `GOFIBER_HOST` - Server host
   - `GOFIBER_PORT` - Server port
   - `GOFIBER_CORS_ORIGINS` - CORS allowed origins

## Project Structure

```
gofiber-api/
├── .releaser.yaml          # Releaser configuration
├── Dockerfile              # Docker build file
├── README.md               # This file
├── config.example.yaml     # Example configuration
├── go.mod                  # Go module file
├── cmd/
│   └── server/
│       └── main.go         # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go       # Configuration handling
│   ├── handlers/
│   │   └── handlers.go     # HTTP request handlers
│   ├── models/
│   │   └── models.go       # Data models
│   └── store/
│       └── store.go        # Data storage layer
└── scripts/
    ├── gofiber-api.service # Systemd service file
    ├── postinstall.sh      # Post-install script
    └── preremove.sh        # Pre-removal script
```

## Packaging

The `.releaser.yaml` configuration includes:

- **Cross-platform builds**: Linux, macOS, Windows (amd64, arm64)
- **Archives**: tar.gz for Linux/macOS, zip for Windows
- **Linux packages**: DEB, RPM, APK
- **Docker images**: Multi-arch (amd64, arm64)
- **Homebrew**: macOS/Linux tap formula
- **Scoop**: Windows bucket manifest
- **AUR**: Arch Linux package

## License

MIT License
