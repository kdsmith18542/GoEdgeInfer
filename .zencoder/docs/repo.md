# GoEdgeInfer Information

## Summary
GoEdgeInfer is a lightweight, high-performance Go-based server for AI/ML model inference, optimized for edge computing environments and serverless functions. It provides fast inference with minimal latency, small memory footprint, quick cold starts, concurrent request handling, built-in Prometheus metrics, and graceful shutdown capabilities.

## Structure
- **cmd/**: Contains the main application entry points
- **configs/**: Configuration files including config.yaml
- **internal/**: Core implementation components (API, middleware, inference engines)
- **pkg/**: Reusable packages for logging and metrics
- **proto/**: Protocol buffer definitions for gRPC services
- **scripts/**: Utility scripts for testing and development
- **models/**: Directory for storing ML models
- **test_models/**: Test models for development and testing
- **docs/**: Project documentation

## Language & Runtime
**Language**: Go
**Version**: 1.23.0 (toolchain 1.23.11)
**Build System**: Go Modules
**Package Manager**: Go Modules

## Dependencies
**Main Dependencies**:
- github.com/gin-gonic/gin v1.10.1 (Web framework)
- github.com/yalue/onnxruntime_go v1.21.0 (ONNX runtime)
- github.com/dgraph-io/badger/v4 v4.7.0 (Key-value store)
- github.com/prometheus/client_golang v1.22.0 (Metrics)
- github.com/spf13/viper v1.20.1 (Configuration)
- github.com/sirupsen/logrus v1.9.3 (Logging)
- google.golang.org/grpc v1.73.0 (gRPC support)
- github.com/minio/minio-go/v7 v7.0.94 (S3 client)
- go.opentelemetry.io/otel v1.37.0 (Telemetry)

## Build & Installation
```bash
# Clone the repository
git clone https://github.com/keith/goedgeinfer.git
cd goedgeinfer

# Install dependencies
go mod download

# Build the application
go build -o bin/goedgeinfer cmd/main.go
```

## Configuration
The application is configured through environment variables and a YAML configuration file:

**Environment Variables**:
- PORT: Server port (default: 8080)
- ENV: Environment (development/production)
- WORKER_POOL_SIZE: Number of worker goroutines (default: 10)
- MODEL_PATH: Path to models directory (default: ./models)
- LOG_LEVEL: Logging level (debug, info, warn, error)

**Config File**: configs/config.yaml contains settings for:
- Server configuration
- Worker pool size
- Model paths
- Logging configuration
- Processing pipeline steps
- S3 storage configuration
- Authentication (API keys, JWT)
- TLS configuration

## Testing
**Framework**: Go's built-in testing package
**Test Location**: Tests are located alongside the code they test
**Naming Convention**: *_test.go files
**Run Command**:
```bash
go test ./...
```

## Features
- ONNX and TFLite model support
- REST and gRPC API interfaces
- JWT and API key authentication
- Rate limiting middleware
- S3 integration for model storage
- Prometheus metrics
- OpenTelemetry tracing
- Processing pipeline for inference results
- Worker pool for concurrent processing