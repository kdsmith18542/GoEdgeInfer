package inference

import (
	"context"
	"sync"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"go.opentelemetry.io/otel/trace"
)

// Prediction represents the result of an inference
type Prediction struct {
	ModelID  string      `json:"model_id"`
	Version  string      `json:"version,omitempty"`
	Output   interface{} `json:"output"`
	Metadata interface{} `json:"metadata,omitempty"`
	Latency  int64       `json:"latency_ms"` // in milliseconds
}

// InputPreprocessor defines the interface for input preprocessing
type InputPreprocessor interface {
	Process(input interface{}) (interface{}, error)
}

// OutputPostprocessor defines the interface for output postprocessing
type OutputPostprocessor interface {
	Process(output interface{}) (interface{}, error)
}

// Engine defines the interface for model inference operations
// All model operations now support version
// Add tracing-aware model load/unload
type Engine interface {
	// LoadModel loads a model into memory
	LoadModel(model *model.Model) error

	// GetModel retrieves a loaded model by ID and version
	GetModel(modelID, version string) (*model.Model, error)

	// UnloadModel unloads a model from memory by ID and version
	UnloadModel(modelID, version string) error

	// Predict performs inference using the specified model
	Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error)

	// BatchPredict performs batch inference using the specified model
	BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error)

	// GetModelInfo returns information about a loaded model
	GetModelInfo(modelID, version string) (*model.Model, error)

	// ListModels returns a list of loaded model IDs and versions
	ListModels() []string

	// LoadModelWithTracing loads a model into memory with tracing
	LoadModelWithTracing(model *model.Model, tracer trace.Tracer) error

	// UnloadModelWithTracing unloads a model from memory by ID and version with tracing
	UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error
}

// BaseEngine provides a base implementation of the Engine interface
// All methods now support version
type BaseEngine struct {
	models       map[string]*model.Model
	modelManager *model.ModelManager
	mu           sync.RWMutex
}

// NewBaseEngine creates a new BaseEngine instance
func NewBaseEngine() *BaseEngine {
	return &BaseEngine{
		models:       make(map[string]*model.Model),
		modelManager: model.NewModelManager(),
	}
}

// GetModel retrieves a loaded model by ID
func (e *BaseEngine) GetModel(modelID, version string) (*model.Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key := modelID
	if version != "" {
		key = modelID + ":" + version
	}
	m, exists := e.models[key]
	if !exists {
		return nil, model.ErrModelNotFound
	}
	return m, nil
}

// LoadModel implements the Engine interface
func (e *BaseEngine) LoadModel(m *model.Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := m.ID
	if m.Version != "" {
		key = m.ID + ":" + m.Version
	}
	if _, exists := e.models[key]; exists {
		return model.ErrModelAlreadyExists
	}
	e.models[key] = m
	return nil
}

// UnloadModel implements the Engine interface
func (e *BaseEngine) UnloadModel(modelID, version string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := modelID
	if version != "" {
		key = modelID + ":" + version
	}
	if _, exists := e.models[key]; !exists {
		return model.ErrModelNotFound
	}
	delete(e.models, key)
	return nil
}

// Predict implements the Engine interface
func (e *BaseEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error) {
	return nil, ErrNotImplemented
}

// BatchPredict implements the Engine interface
func (e *BaseEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	return nil, ErrNotImplemented
}

// GetModelInfo implements the Engine interface
func (e *BaseEngine) GetModelInfo(modelID, version string) (*model.Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key := modelID
	if version != "" {
		key = modelID + ":" + version
	}
	m, exists := e.models[key]
	if !exists {
		return nil, model.ErrModelNotFound
	}
	return m, nil
}

// ListModels implements the Engine interface
func (e *BaseEngine) ListModels() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	modelIDs := make([]string, 0, len(e.models))
	for modelID := range e.models {
		modelIDs = append(modelIDs, modelID)
	}
	return modelIDs
}

// LoadModelWithTracing loads a model and records tracing/metrics
func (e *BaseEngine) LoadModelWithTracing(m *model.Model, tracer trace.Tracer) error {
	_, err := e.modelManager.LoadModel(m.ID, m.Version, m.Path, m.InputShape, m.OutputShape, m.Metadata, tracer)
	return err
}

// UnloadModelWithTracing unloads a model and records tracing/metrics
func (e *BaseEngine) UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error {
	return e.modelManager.UnloadModel(modelID, version, tracer)
}

// MockEngine is a mock implementation of the Engine interface for testing
type MockEngine struct{}

func (m *MockEngine) LoadModel(model *model.Model) error                     { return nil }
func (m *MockEngine) GetModel(modelID, version string) (*model.Model, error) { return nil, nil }
func (m *MockEngine) UnloadModel(modelID, version string) error              { return nil }
func (m *MockEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error) {
	return &Prediction{ModelID: modelID, Output: input, Latency: 1}, nil
}
func (m *MockEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	return nil, nil
}
func (m *MockEngine) GetModelInfo(modelID, version string) (*model.Model, error)         { return nil, nil }
func (m *MockEngine) ListModels() []string                                               { return []string{"test_model"} }
func (m *MockEngine) LoadModelWithTracing(model *model.Model, tracer trace.Tracer) error { return nil }
func (m *MockEngine) UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error {
	return nil
}

// Errors
var (
	ErrNotImplemented      = NewInferenceError("not implemented")
	ErrModelVersionNotFound = NewInferenceError("model version not found")
)

// InferenceError represents an error that occurs during inference
type InferenceError struct {
	msg string
}

// NewInferenceError creates a new InferenceError
func NewInferenceError(msg string) *InferenceError {
	return &InferenceError{msg: msg}
}

// Error implements the error interface
func (e *InferenceError) Error() string {
	return e.msg
}
