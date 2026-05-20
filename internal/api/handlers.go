package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/internal/processing"
	"github.com/kdsmith18542/GoEdgeInfer/internal/worker"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Request/Response Types
type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// LoadModelRequest represents a request to load a model
type LoadModelRequest struct {
	ModelID   string `json:"model_id" binding:"required"`
	ModelPath string `json:"model_path" binding:"required"`
	Version   string `json:"version,omitempty"`
}

// InferenceRequest represents a request for model inference
type InferenceRequest struct {
	// ModelID is the unique identifier for the model
	ModelID string `json:"model_id"`
	// Version is the specific version of the model to use (optional)
	Version string      `json:"version,omitempty"`
	Input   interface{} `json:"input"`
}

// InferenceResponse represents the response from an inference request
type InferenceResponse struct {
	// ModelID is the unique identifier for the model used
	ModelID string `json:"model_id"`
	// Version is the specific version of the model used
	Version string `json:"version,omitempty"`
	// Output is the model's prediction output
	Output interface{} `json:"output"`
	// Latency is the total processing time in milliseconds
	Latency int64 `json:"latency_ms"`
	// Metadata contains additional information about the inference
	Metadata interface{} `json:"metadata,omitempty"`
}

// ReloadRequest is empty for now, but can be extended
// ReloadResponse returns status
type ReloadResponse struct {
	Status string `json:"status"`
}

// RemoteModelManager defines S3/model management operations for testability
// (or write a manual mock for tests)
//
//go:generate mockgen -destination=mock_remote_model_manager.go -package=api github.com/kdsmith18542/GoEdgeInfer/internal/api RemoteModelManager
type RemoteModelManager interface {
	ListRemoteModels(ctx context.Context, cfg *config.Config) ([]string, error)
	CleanupLocalModelCache(cacheDir string, keep map[string]struct{}) error
	DeleteModelFromS3(ctx context.Context, cfg *config.Config, objectKey string) error
	UploadModelToS3(ctx context.Context, cfg *config.Config, localPath, objectKey string) error
}

type defaultRemoteModelManager struct{}

func (d *defaultRemoteModelManager) ListRemoteModels(ctx context.Context, cfg *config.Config) ([]string, error) {
	return model.ListRemoteModelsS3(ctx, cfg)
}
func (d *defaultRemoteModelManager) CleanupLocalModelCache(cacheDir string, keep map[string]struct{}) error {
	return model.CleanupLocalModelCache(cacheDir, keep)
}
func (d *defaultRemoteModelManager) DeleteModelFromS3(ctx context.Context, cfg *config.Config, objectKey string) error {
	return model.DeleteModelFromS3(ctx, cfg, objectKey)
}
func (d *defaultRemoteModelManager) UploadModelToS3(ctx context.Context, cfg *config.Config, localPath, objectKey string) error {
	return model.UploadModelToS3(ctx, cfg, localPath, objectKey)
}

// API represents the API server
type API struct {
	router        *gin.Engine
	engine        inference.Engine
	workerPool    *worker.WorkerPool // Updated type
	tracer        trace.Tracer
	logger        *zap.Logger
	modelManager  ModelManager
	remoteManager RemoteModelManager // Added remote manager
	pipeline      *processing.Pipeline
}

// auditLog logs an audit event with the given action and details
func (a *API) auditLog(c *gin.Context, action string, details map[string]interface{}) {
	// Add request ID if available
	if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
		details["request_id"] = requestID
	}

	// Add client IP if available
	if clientIP := c.ClientIP(); clientIP != "" {
		details["client_ip"] = clientIP
	}

	// Prepare log fields with the action
	fields := []interface{}{"action", action}
	// Add details as key-value pairs
	for k, v := range details {
		fields = append(fields, k, v)
	}

	// Log with all fields
	a.logger.Info("audit", zap.String("action", action), zap.Any("details", details))
}

// ModelManager defines the interface for model management operations
type ModelManager interface {
	ListModels() []string
	GetModelInfo(modelID, version string) (interface{}, error)
	LoadModel(ctx context.Context, modelID, version, path string) error
	UnloadModel(ctx context.Context, modelID, version string) error
}

// NewAPI creates a new API instance
func NewAPI(engine inference.Engine, workerPool *worker.WorkerPool, modelMgr ModelManager, tracer trace.Tracer, logger *zap.Logger) *API {
	api := &API{
		router:       gin.New(),
		engine:       engine,
		workerPool:   workerPool,
		modelManager: modelMgr,
		tracer:       tracer,
		logger:       logger,
	}
	return api
}

// HealthCheck handles the health check endpoint
func (a *API) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// Infer handles the inference requests
func (a *API) Infer(c *gin.Context) {
	ctx, span := a.tracer.Start(c.Request.Context(), "API.Infer")
	defer span.End()

	_ = ctx // ctx is not used directly, but required for span

	startTime := time.Now()

	// Read and parse request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read request body"})
		return
	}

	var req InferenceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request format"})
		return
	}

	if req.ModelID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "model_id is required"})
		return
	}

	// Pre-processing pipeline
	processedInput := req.Input
	if a.pipeline != nil {
		var pipeErr error
		processedInput, pipeErr = a.pipeline.Run(req.Input)
		if pipeErr != nil {
			span.RecordError(pipeErr)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "pre-processing failed: " + pipeErr.Error()})
			return
		}
	}

	// Submit task to worker pool
	resultCh, errCh := a.workerPool.Submit(req.ModelID, req.Version, processedInput)

	select {
	case result := <-resultCh:
		// Post-processing pipeline
		output := result.Output
		if a.pipeline != nil {
			var pipeErr error
			output, pipeErr = a.pipeline.Run(result.Output)
			if pipeErr != nil {
				span.RecordError(pipeErr)
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "post-processing failed: " + pipeErr.Error()})
				return
			}
		}
		// Update metrics (per-model)
		metrics.InferenceRequestCounter.WithLabelValues(req.ModelID).Inc()
		metrics.RecordInferenceDuration(req.ModelID, float64(time.Since(startTime).Milliseconds())/1000.0)
		span.SetAttributes(attribute.String("model_id", req.ModelID))
		c.JSON(http.StatusOK, InferenceResponse{
			ModelID:  result.ModelID,
			Output:   output,
			Latency:  result.Latency,
			Metadata: result.Metadata,
		})

	case err := <-errCh:
		metrics.InferenceErrorsCounter.WithLabelValues(req.ModelID).Inc()
		a.logger.Error("Inference failed", zap.Error(err))
		span.RecordError(err)
		status := http.StatusInternalServerError
		errMsg := err.Error()
		switch err {
		case model.ErrModelNotFound:
			status = http.StatusNotFound
		case inference.ErrNotImplemented:
			status = http.StatusNotImplemented
		}
		c.JSON(status, ErrorResponse{Error: errMsg})

	case <-time.After(30 * time.Second):
		span.RecordError(context.DeadlineExceeded)
		c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "inference request timed out"})
	}
}

// LoadModel handles loading a new model
func (a *API) LoadModel(c *gin.Context) {
	var req LoadModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Check if model already exists (version-aware)
	if _, err := a.engine.GetModel(req.ModelID, req.Version); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "model with this ID and version already exists"})
		return
	}

	// Create and load the model
	model := &model.Model{
		ID:      req.ModelID,
		Version: req.Version,
		Path:    req.ModelPath,
	}

	// Use model manager with tracing
	if err := a.engine.LoadModelWithTracing(model, a.tracer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"model_id": req.ModelID,
		"version":  req.Version,
		"status":   "model_loaded",
	})
}

// Predict handles prediction requests with support for model versioning
// @Summary Perform inference with a model
// @Description Run inference using the specified model and version
// @Tags inference
// @Accept json
// @Produce json
// @Param model_id path string true "Model ID"
// @Param request body InferenceRequest true "Inference request"
// @Success 200 {object} InferenceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /predict/{model_id} [post]
func (a *API) Predict(c *gin.Context) {
	startTime := time.Now()
	ctx, span := a.tracer.Start(c.Request.Context(), "API.Predict")
	defer span.End()

	// Get model ID from URL path
	modelID := c.Param("model_id")
	if modelID == "" {
		err := fmt.Errorf("model_id is required")
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":    err.Error(),
			"model_id": modelID,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Parse request body
	var req InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := fmt.Sprintf("invalid request: %v", err)
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":    errMsg,
			"model_id": modelID,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
		return
	}

	// Ensure the model ID in the path matches the one in the request body if provided
	if req.ModelID != "" && req.ModelID != modelID {
		errMsg := "model_id in path does not match model_id in request body"
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":         errMsg,
			"path_model_id": modelID,
			"body_model_id": req.ModelID,
			"model_version": req.Version,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
		return
	}

	// Set model ID from path if not provided in request body
	if req.ModelID == "" {
		req.ModelID = modelID
	}

	// Validate input
	if req.Input == nil {
		errMsg := "input is required"
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":         errMsg,
			"model_id":      req.ModelID,
			"model_version": req.Version,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
		return
	}

	// Convert input to []float32 for pipeline compatibility
	processedInput, err := convertToFloat32Slice(req.Input)
	if err != nil {
		errMsg := fmt.Sprintf("input conversion failed: %v", err)
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":         errMsg,
			"model_id":      req.ModelID,
			"model_version": req.Version,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: errMsg})
		return
	}

	// Log the inference request
	a.auditLog(c, "inference_start", map[string]interface{}{
		"model_id":      req.ModelID,
		"model_version": req.Version,
	})

	// Submit task to worker pool with version
	resultCh, errCh := a.workerPool.Submit(req.ModelID, req.Version, processedInput)

	// Wait for result or error
	select {
	case result := <-resultCh:
		if result == nil {
			errMsg := "no result from worker"
			a.auditLog(c, "inference_error", map[string]interface{}{
				"error":         errMsg,
				"model_id":      req.ModelID,
				"model_version": req.Version,
			})
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: errMsg})
			return
		}

		// Update metrics
		metrics.InferenceRequestCounter.WithLabelValues(req.ModelID).Inc()
		metrics.RecordInferenceDuration(req.ModelID, float64(time.Since(startTime).Milliseconds())/1000.0)

		// Log successful inference
		a.auditLog(c, "inference_success", map[string]interface{}{
			"model_id":      req.ModelID,
			"model_version": req.Version,
			"latency_ms":    time.Since(startTime).Milliseconds(),
		})

		c.JSON(http.StatusOK, InferenceResponse{
			ModelID:  result.ModelID,
			Version:  result.Version,
			Output:   result.Output,
			Latency:  result.Latency,
			Metadata: result.Metadata,
		})

	case err := <-errCh:
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, model.ErrModelNotFound):
			status = http.StatusNotFound
		case errors.Is(err, inference.ErrModelVersionNotFound):
			status = http.StatusNotFound
		case errors.Is(err, context.DeadlineExceeded):
			status = http.StatusGatewayTimeout
		}

		// Log the error
		a.auditLog(c, "inference_error", map[string]interface{}{
			"error":         err.Error(),
			"model_id":      req.ModelID,
			"model_version": req.Version,
			"status_code":   status,
		})

		c.JSON(status, ErrorResponse{Error: err.Error()})

	case <-ctx.Done():
		// Request was cancelled
		a.auditLog(c, "inference_cancelled", map[string]interface{}{
			"model_id":      req.ModelID,
			"model_version": req.Version,
			"reason":        ctx.Err().Error(),
		})
	}
}

// UnloadModel handles model unloading requests
func (a *API) UnloadModel(c *gin.Context) {
	modelID := c.Param("model_id")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "model id is required"})
		return
	}

	if err := a.engine.UnloadModelWithTracing(modelID, "", a.tracer); err != nil {
		status := http.StatusInternalServerError
		if err == model.ErrModelNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("model %s unloaded successfully", modelID),
	})
}

// GetModelInfo returns information about a loaded model
func (a *API) GetModelInfo(c *gin.Context) {
	modelID := c.Param("model_id")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "model_id is required"})
		return
	}

	version := c.Param("version")

	// Get model info from the engine
	info, err := a.engine.GetModelInfo(modelID, version)
	if err != nil {
		a.auditLog(c, "model_info_error", map[string]interface{}{
			"error":    err.Error(),
			"model_id": modelID,
			"version":  version,
		})
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	a.auditLog(c, "model_info", map[string]interface{}{
		"model_id": modelID,
		"version":  version,
	})

	c.JSON(http.StatusOK, info)
}

// BatchPredict handles batch prediction requests
func (a *API) BatchPredict(c *gin.Context) {
	startTime := time.Now()
	ctx, span := a.tracer.Start(c.Request.Context(), "API.BatchPredict")
	defer span.End()

	modelID := c.Param("model_id")
	if modelID == "" {
		a.auditLog(c, "batch_predict_error", map[string]interface{}{
			"error":    "model_id is required",
			"model_id": modelID,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "model_id is required"})
		return
	}

	// Get version from path or query parameter
	version := c.Param("version")
	if version == "" {
		version = c.Query("version")
	}

	var req BatchInferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.auditLog(c, "batch_predict_error", map[string]interface{}{
			"error":    fmt.Sprintf("invalid request: %v", err),
			"model_id": modelID,
			"version":  version,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// Validate batch size
	if len(req.Inputs) == 0 {
		a.auditLog(c, "batch_predict_error", map[string]interface{}{
			"error":    "no inputs provided",
			"model_id": modelID,
			"version":  version,
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "no inputs provided"})
		return
	}

	// Process batch prediction
	results := make([]interface{}, 0, len(req.Inputs))
	for _, input := range req.Inputs {
		// Create a prediction task
		task := &worker.Task{
			ModelID:      modelID,
			ModelVersion: version,
			Input:        input,
		}

		// Submit task to worker pool
		resultCh, errCh := a.workerPool.Submit(modelID, version, task)

		// Wait for result or error
		select {
		case result := <-resultCh:
			if result == nil {
				errMsg := "no result from worker"
				a.auditLog(c, "batch_predict_error", map[string]interface{}{
					"error":         errMsg,
					"model_id":      modelID,
					"model_version": version,
				})
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: errMsg})
				return
			}
			results = append(results, result)
		case err := <-errCh:
			a.auditLog(c, "batch_predict_error", map[string]interface{}{
				"error":    err.Error(),
				"model_id": modelID,
				"version":  version,
			})
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		case <-ctx.Done():
			// Request was cancelled
			a.auditLog(c, "batch_predict_cancelled", map[string]interface{}{
				"model_id":      modelID,
				"model_version": version,
				"reason":        ctx.Err().Error(),
			})
			return
		}
	}

	// Calculate total latency
	latency := time.Since(startTime).Milliseconds()

	a.auditLog(c, "batch_predict_success", map[string]interface{}{
		"model_id":   modelID,
		"version":    version,
		"batch_size": len(req.Inputs),
		"latency_ms": latency,
		"status":     "success",
	})

	c.JSON(http.StatusOK, BatchInferenceResponse{
		ModelID:   modelID,
		Version:   version,
		Outputs:   results,
		LatencyMs: latency,
		BatchSize: len(req.Inputs),
	})
}

// ListModels handles listing all loaded models
func (a *API) ListModels(c *gin.Context) {
	models := a.modelManager.ListModels()
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// Reload handles configuration reload requests
func (a *API) Reload(c *gin.Context) {
	// In a real implementation, this would reload the configuration
	// and update the server state accordingly
	c.JSON(http.StatusOK, gin.H{"status": "reloaded"})
}

// ListRemoteModels lists all available remote models
func (a *API) ListRemoteModels(c *gin.Context) {
	if !requireRole(c, "admin", "ops") {
		return
	}
	a.auditLog(c, "list_remote_models", nil)

	cfg := config.Load() // or inject config if available
	models, err := a.remoteManager.ListRemoteModels(c.Request.Context(), cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"remote_models": models})
}

// CleanupModelCache cleans up the local model cache
func (a *API) CleanupModelCache(c *gin.Context) {
	var req CleanupCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	keepSet := make(map[string]struct{})
	for _, p := range req.Keep {
		keepSet[p] = struct{}{}
	}
	err := a.remoteManager.CleanupLocalModelCache(req.CacheDir, keepSet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cache_cleaned"})
}

// DeleteRemoteModel handles deleting a model from the remote S3/MinIO registry
func (a *API) DeleteRemoteModel(c *gin.Context) {
	var req DeleteRemoteModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	cfg := config.Load()
	err := a.remoteManager.DeleteModelFromS3(c.Request.Context(), cfg, req.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "remote_model_deleted"})
}

// UploadRemoteModel handles uploading a model to the remote S3/MinIO registry
func (a *API) UploadRemoteModel(c *gin.Context) {
	var req UploadRemoteModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	cfg := config.Load()
	err := a.remoteManager.UploadModelToS3(c.Request.Context(), cfg, req.LocalPath, req.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "remote_model_uploaded"})
}

// RBAC check helper
func requireRole(c *gin.Context, allowedRoles ...string) bool {
	claims, exists := c.Get("jwt_claims")
	if !exists {
		// If JWT claims are missing, it means JWT auth is disabled; allow request
		return true
	}
	role, ok := claims.(map[string]interface{})["role"].(string)
	if !ok {
		c.AbortWithStatusJSON(403, gin.H{"error": "missing role claim"})
		return false
	}
	for _, allowed := range allowedRoles {
		if role == allowed {
			return true
		}
	}
	c.AbortWithStatusJSON(403, gin.H{"error": "insufficient role"})
	return false
}

// Audit log helper
func auditLog(c *gin.Context, action string, details interface{}) {
	user := "unknown"
	if claims, exists := c.Get("jwt_claims"); exists {
		if sub, ok := claims.(map[string]interface{})["sub"].(string); ok {
			user = sub
		}
	}
	logging.Info("AUDIT", zap.String("user", user), zap.String("action", action), zap.Any("details", details))
}

type CleanupCacheRequest struct {
	CacheDir string   `json:"cache_dir"`
	Keep     []string `json:"keep"`
}

type DeleteRemoteModelRequest struct {
	ObjectKey string `json:"object_key" binding:"required"`
}

type BatchInferenceRequest struct {
	Inputs []interface{} `json:"inputs" binding:"required"`
}

type BatchInferenceResponse struct {
	ModelID   string        `json:"model_id"`
	Version   string        `json:"version"`
	Outputs   []interface{} `json:"outputs"`
	LatencyMs int64         `json:"latency_ms"`
	BatchSize int           `json:"batch_size"`
}

type UploadRemoteModelRequest struct {
	LocalPath string `json:"local_path" binding:"required"`
	ObjectKey string `json:"object_key" binding:"required"`
}

// convertToFloat32Slice converts various input types to []float32
func convertToFloat32Slice(input interface{}) ([]float32, error) {
	switch v := input.(type) {
	case []float32:
		return v, nil
	case []float64:
		result := make([]float32, len(v))
		for i, val := range v {
			result[i] = float32(val)
		}
		return result, nil
	case []interface{}:
		result := make([]float32, len(v))
		for i, val := range v {
			switch num := val.(type) {
			case float64:
				result[i] = float32(num)
			case float32:
				result[i] = num
			case int:
				result[i] = float32(num)
			case int64:
				result[i] = float32(num)
			default:
				return nil, fmt.Errorf("unsupported type at index %d: %T", i, val)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}
}
