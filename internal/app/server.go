package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kdsmith18542/GoEdgeInfer/internal/api"
	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
	"github.com/kdsmith18542/GoEdgeInfer/internal/grpcapi"
	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/persistence"
	"github.com/kdsmith18542/GoEdgeInfer/internal/processing"
	"github.com/kdsmith18542/GoEdgeInfer/internal/worker"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
	"github.com/kdsmith18542/GoEdgeInfer/proto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Server struct {
	HTTPServer   *http.Server
	GRPCAddr     string
	Tracer       *sdktrace.TracerProvider
	ShutdownFunc func()
	WorkerPool   *worker.WorkerPool
	ReloadCh     chan struct{}
	ServerErrors chan error
	ShutdownCh   chan os.Signal
	Config       *config.Config
	Logger       *zap.Logger
}

// newExporter returns a console exporter.
func newExporter(w io.Writer) (sdktrace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithoutTimestamps(),
	)
}

// InitTracer initializes and returns a TracerProvider
func InitTracer() (*sdktrace.TracerProvider, error) {
	// Create stdout exporter to be able to retrieve
	// the collected spans.
	exp, err := newExporter(os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("goedgeinfer"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create the TracerProvider with the exporter and resource
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	// Set the global TracerProvider
	otel.SetTracerProvider(tp)

	// Set up propagation
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return tp, nil
}

// NewDefaultModelManager creates a new default model manager
func NewDefaultModelManager(engine inference.Engine) api.ModelManager {
	return &defaultModelManager{
		engine: engine,
	}
}

type defaultModelManager struct {
	engine inference.Engine
}

// ListModels returns a list of all loaded model IDs
func (m *defaultModelManager) ListModels() []string {
	// Return a slice of model IDs as strings
	// This is a simplified implementation - in a real scenario, you'd get this from the engine
	return []string{"model1", "model2"}
}

// GetModelInfo returns information about a specific model
func (m *defaultModelManager) GetModelInfo(modelID, version string) (interface{}, error) {
	// In a real implementation, you would fetch this from your model storage
	// This is a simplified example
	return map[string]interface{}{
		"model_id": modelID,
		"version":  version,
		"status":   "loaded",
	}, nil
}

// LoadModel loads a model with the given ID and version from the specified path
func (m *defaultModelManager) LoadModel(ctx context.Context, modelID, version, path string) error {
	// Create a new span for tracing
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "LoadModel")
	defer span.End()

	// In a real implementation, you would load the model here
	// For now, we'll just log that we're loading the model
	return nil
}

// UnloadModel unloads a model with the given ID and version
func (m *defaultModelManager) UnloadModel(ctx context.Context, modelID, version string) error {
	// Create a new span for tracing
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "UnloadModel")
	defer span.End()

	// In a real implementation, you would unload the model here
	// For now, we'll just log that we're unloading the model
	return nil
}

// ListRemoteModels lists all available remote models
func (m *defaultModelManager) ListRemoteModels() ([]string, error) {
	// In a real implementation, you would list available remote models
	return []string{"remote_model1", "remote_model2"}, nil
}

// CleanupModelCache cleans up the model cache
func (m *defaultModelManager) CleanupModelCache() error {
	// In a real implementation, you would clean up the model cache
	return nil
}

// DeleteRemoteModel deletes a remote model
func (m *defaultModelManager) DeleteRemoteModel(modelID, version string) error {
	// In a real implementation, you would delete the remote model
	return nil
}

// UploadRemoteModel uploads a model to the remote storage
func (m *defaultModelManager) UploadRemoteModel(modelID, version, path string) error {
	// In a real implementation, you would upload the model to remote storage
	return nil
}

func NewServerWithConfig(cfg *config.Config) (*Server, error) {
	queuePath := os.Getenv("GOEDGEINFER_QUEUE_PATH")
	if queuePath == "" {
		queuePath = "./data/queue"
	}
	grpcAddr := os.Getenv("GOEDGEINFER_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":50051"
	}

	tp, err := InitTracer()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize OpenTelemetry tracer: %w", err)
	}
	tracer := otel.Tracer("goedgeinfer")

	pipeline, err := processing.NewPipelineFromConfig(cfg.Pipeline)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize processing pipeline: %w", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	logging.Info("Initializing ONNX Runtime inference engine...")
	engine, err := inference.NewONNXRuntimeEngine()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize ONNX Runtime: %w", err)
	}

	queue, err := persistence.NewPersistentQueue(queuePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize persistent queue: %w", err)
	}

	workerPool := worker.NewWorkerPool(engine, cfg.WorkerPoolSize, queue, pipeline, tracer)
	workerPool.RecoverFromPersistentQueue()

	reloadCh := make(chan struct{}, 1)

	// Create a default model manager
	modelMgr := &defaultModelManager{engine: engine}

	// Initialize logger
	logger, err = zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	// Create API instance with all required dependencies
	apiInstance := api.NewAPI(engine, workerPool, modelMgr, tracer, logger)
	r := gin.Default()
	api.SetupRoutes(r, apiInstance, cfg.APIKey, cfg.JWT, logger)
	r.POST("/reload", func(c *gin.Context) {
		select {
		case reloadCh <- struct{}{}:
			c.JSON(http.StatusOK, gin.H{"status": "reload_triggered"})
		default:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "reload already in progress"})
		}
	})

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	if cfg.TLS.Enabled {
		server := srv
		server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if cfg.TLS.RequireClientCert {
			caCert, err := os.ReadFile(cfg.TLS.ClientCA)
			if err != nil {
				return nil, fmt.Errorf("Failed to read client CA cert: %w", err)
			}
			caPool := x509.NewCertPool()
			caPool.AppendCertsFromPEM(caCert)
			server.TLSConfig.ClientCAs = caPool
			server.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		go func() {
			serverErrors <- server.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		}()
	} else {
		go func() {
			logger.Info(fmt.Sprintf("Server listening on %s", srv.Addr))
			serverErrors <- srv.ListenAndServe()
		}()
	}

	// gRPC server
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen for gRPC: %w", err)
	}

	go func() {
		grpcServer := grpc.NewServer()
		proto.RegisterGoEdgeInferServiceServer(grpcServer, grpcapi.NewServer(engine, workerPool))
		logger.Info(fmt.Sprintf("gRPC server listening on %s", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	return &Server{
		HTTPServer:   srv,
		GRPCAddr:     grpcAddr,
		Tracer:       tp,
		ShutdownFunc: func() { engine.Close(); queue.Close(); lis.Close() },
		WorkerPool:   workerPool,
		ReloadCh:     reloadCh,
		ServerErrors: serverErrors,
		ShutdownCh:   shutdownCh,
		Config:       cfg,
		Logger:       logger,
	}, nil
}

func NewServer() (*Server, error) {
	cfg := config.Load()
	return NewServerWithConfig(cfg)
}

func (s *Server) Start() {
	for {
		select {
		case err := <-s.ServerErrors:
			s.Logger.Fatal(fmt.Sprintf("Server error: %v", err))
		case sig := <-s.ShutdownCh:
			s.Logger.Info(fmt.Sprintf("Received %v signal. Starting graceful shutdown...", sig))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.HTTPServer.Shutdown(ctx); err != nil {
				s.ShutdownCh <- os.Interrupt
				s.Logger.Error(fmt.Sprintf("Graceful shutdown did not complete in 30s: %v", err))
			}
			s.WorkerPool.Shutdown()
			s.Logger.Info("Server gracefully stopped")
			return
		case <-s.ReloadCh:
			s.Logger.Info("Reloading configuration and pipeline...")
			cfg := config.Load()
			newPipeline, err := processing.NewPipelineFromConfig(cfg.Pipeline)
			if err != nil {
				s.Logger.Error("Failed to reload processing pipeline", zap.Error(err))
			} else {
				s.WorkerPool.UpdatePipeline(newPipeline)
				s.Logger.Info("Pipeline reloaded successfully")
			}
		}
	}
}
