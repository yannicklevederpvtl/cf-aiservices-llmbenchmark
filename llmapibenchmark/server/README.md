# LLM Benchmark HTTP Server

This package provides an HTTP server wrapper around the existing `llmapibenchmark` codebase, exposing REST API endpoints for benchmarking LLM models.

## Architecture

```
server/
├── handlers.go      # HTTP request handlers
├── middleware.go    # CORS, logging, and other middleware
├── routes.go        # Route definitions and setup
└── README.md        # This file

cmd/server/
└── main.go          # Server entry point
```

## Running the Server

### Development Mode

```bash
# Build the server
go build -o bin/server cmd/server/main.go

# Run the server
./bin/server

# Or with custom port
PORT=8080 ./bin/server
```

### Production Mode

```bash
# Set production mode
export GIN_MODE=release
export PORT=8080

# Run the server
./bin/server
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `GIN_MODE` | `debug` | Gin mode (`debug` or `release`) |

## API Endpoints

### Health Check
```
GET /api/health

Response:
{
  "status": "ok",
  "version": "1.0.0",
  "timestamp": "2025-10-14T20:38:11Z"
}
```

### Model Discovery
```
GET /api/models

Response:
{
  "message": "Models endpoint - to be implemented",
  "models": []
}
```

### Run Benchmark
```
POST /api/benchmark

Request:
{
  "model1": { "id": "gpt-4", "name": "GPT-4", ... },
  "model2": { "id": "claude-3", "name": "Claude 3", ... },
  "concurrency": 5,
  "maxTokens": 100,
  "prompt": "Write a story..."
}

Response: (To be implemented in task 8.2)
```

### Export Results
```
POST /api/export/json
POST /api/export/csv

(To be implemented in task 8.3)
```

## Features

✅ **Implemented (Task 8.1):**
- Basic HTTP server with Gin framework
- Graceful shutdown handling
- CORS middleware for development
- Request logging middleware
- Health check endpoint
- Route structure and organization

⏳ **Coming Next:**
- Task 8.2: Core API endpoints (models, benchmark)
- Task 8.3: Export endpoints (JSON, CSV)
- Task 8.4: Enhanced error handling
- Task 8.5: Static file serving for Vue.js
- Task 8.6: Integration testing

## Testing

```bash
# Test health endpoint
curl http://localhost:8080/api/health

# Test models endpoint
curl http://localhost:8080/api/models

# Test root endpoint
curl http://localhost:8080/
```

## Middleware

### CORS Middleware
- Allows all origins (`*`)
- Supports credentials
- Handles preflight OPTIONS requests
- Allows common HTTP methods

### Logging Middleware
- Logs all requests with method, URI, client IP, status, and duration
- Provides detailed timing information for performance monitoring

## Next Steps

1. Implement model discovery from environment/VCAP_SERVICES (Task 8.2)
2. Integrate existing benchmark logic (Task 8.2)
3. Add export functionality (Task 8.3)
4. Enhance error handling (Task 8.4)
5. Configure static file serving (Task 8.5)
6. Add comprehensive testing (Task 8.6)

