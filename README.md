# GoEdgeInfer

A lightweight, high-performance Go-based server for AI/ML model inference, optimized for edge computing environments and serverless functions.

## Features

- 🚀 Fast inference with minimal latency
- 📦 Small memory footprint for edge devices
- ⚡ Quick cold starts for serverless environments
- 🔄 Concurrent request handling with Go's goroutines
- 📊 Built-in Prometheus metrics
- 🛡️ Graceful shutdown and health checks

## Getting Started

### Prerequisites

- Go 1.20 or higher
- Git

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/keith/goedgeinfer.git
   cd goedgeinfer
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

### Running the Server

```bash
export PORT=8080
go run cmd/main.go
```

The server will start on `http://localhost:8080` by default.

## API Endpoints

- `GET /api/v1/health` - Health check endpoint
- `GET /metrics` - Prometheus metrics
- `POST /api/v1/infer` - Perform inference (not implemented yet)
- `POST /api/v1/models/load` - Load a model (not implemented yet)

## Configuration

Environment variables:

- `PORT` - Port to run the server on (default: 8080)
- `ENV` - Environment (development/production) (default: development)
- `WORKER_POOL_SIZE` - Number of worker goroutines (default: 10)
- `MODEL_PATH` - Path to directory containing models (default: ./models)
- `LOG_LEVEL` - Logging level (debug, info, warn, error) (default: info)

## Development

### Building

```bash
go build -o bin/goedgeinfer cmd/main.go
```

### Running Tests

```bash
go test ./...
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
