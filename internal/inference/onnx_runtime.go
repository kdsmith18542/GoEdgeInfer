package inference

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
	ort "github.com/yalue/onnxruntime_go"
	"go.opentelemetry.io/otel/trace"
)

// ONNXModel represents a loaded ONNX model with its session and metadata
// Uses float32 for input/output data
type ONNXModel struct {
	modelID     string
	modelPath   string
	session     *ort.AdvancedSession
	inputNames  []string
	outputNames []string
	inputDims   [][]int64
	outputDims  [][]int64
	inputs      []ort.Value // input tensors (as interface)
	outputs     []ort.Value // output tensors (as interface)
	metadata    map[string]interface{}
}

// ONNXRuntimeEngine is an implementation of the Engine interface using ONNX Runtime
type ONNXRuntimeEngine struct {
	*BaseEngine
	models map[string]*ONNXModel
	mu     sync.RWMutex
}

// NewONNXRuntimeEngine creates a new ONNX Runtime engine instance
func NewONNXRuntimeEngine() (*ONNXRuntimeEngine, error) {
	logging.Info("Initializing ONNX Runtime engine")

	// Set the path to the ONNX Runtime shared library
	onnxRuntimePath := filepath.Join(os.Getenv("HOME"), "onnxruntime/onnxruntime-linux-x64-1.22.0/lib/libonnxruntime.so")
	ort.SetSharedLibraryPath(onnxRuntimePath)

	// Initialize the environment (required for GetInputOutputInfo)
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX environment: %w", err)
		}
	}

	return &ONNXRuntimeEngine{
		BaseEngine: NewBaseEngine(),
		models:     make(map[string]*ONNXModel),
	}, nil
}

// Close releases all resources used by the ONNXRuntimeEngine
func (e *ONNXRuntimeEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clean up all model sessions
	for _, model := range e.models {
		if model.session != nil {
			model.session.Destroy()
		}
		// Destroy input/output tensors
		for _, t := range model.inputs {
			t.Destroy()
		}
		for _, t := range model.outputs {
			t.Destroy()
		}
	}

	// Clear the models map
	e.models = make(map[string]*ONNXModel)

	logging.Info("ONNX Runtime engine closed")
	return nil
}

// unloadModel unloads a model without acquiring a lock
func (e *ONNXRuntimeEngine) unloadModel(modelID string) error {
	if model, exists := e.models[modelID]; exists {
		if model.session != nil {
			model.session.Destroy()
		}
		for _, t := range model.inputs {
			t.Destroy()
		}
		for _, t := range model.outputs {
			t.Destroy()
		}
		delete(e.models, modelID)
		logging.Info("Unloaded ONNX model", "model_id", modelID)
	}
	return nil
}

// LoadModel loads an ONNX model from the given path
func (e *ONNXRuntimeEngine) LoadModel(m *model.Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := m.ID
	if m.Version != "" {
		key = m.ID + ":" + m.Version
	}

	// Check if model is already loaded
	if _, exists := e.models[key]; exists {
		return fmt.Errorf("model with ID %s is already loaded", key)
	}

	// Verify model file exists
	if _, err := os.Stat(m.Path); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", m.Path)
	}

	// Get input/output info
	inputInfos, outputInfos, err := ort.GetInputOutputInfo(m.Path)
	if err != nil {
		return fmt.Errorf("failed to get input/output info: %w", err)
	}
	inputNames := make([]string, len(inputInfos))
	inputDims := make([][]int64, len(inputInfos))
	inputs := make([]ort.Value, len(inputInfos))
	for i, info := range inputInfos {
		inputNames[i] = info.Name
		inputDims[i] = info.Dimensions
		// Allocate empty tensor for input (will be filled at inference time)
		// Use float32 for now; extend for other types if needed
		inTensor, err := ort.NewTensor[float32](info.Dimensions, make([]float32, info.Dimensions.FlattenedSize()))
		if err != nil {
			return fmt.Errorf("failed to create input tensor: %w", err)
		}
		inputs[i] = inTensor
	}
	outputNames := make([]string, len(outputInfos))
	outputDims := make([][]int64, len(outputInfos))
	outputs := make([]ort.Value, len(outputInfos))
	for i, info := range outputInfos {
		outputNames[i] = info.Name
		outputDims[i] = info.Dimensions
		outTensor, err := ort.NewEmptyTensor[float32](info.Dimensions)
		if err != nil {
			return fmt.Errorf("failed to create output tensor: %w", err)
		}
		outputs[i] = outTensor
	}

	// Create AdvancedSession
	session, err := ort.NewAdvancedSession(m.Path, inputNames, outputNames, inputs, outputs, nil)
	if err != nil {
		return fmt.Errorf("failed to create ONNX AdvancedSession: %w", err)
	}

	e.models[key] = &ONNXModel{
		modelID:     m.ID,
		modelPath:   m.Path,
		session:     session,
		inputNames:  inputNames,
		outputNames: outputNames,
		inputDims:   inputDims,
		outputDims:  outputDims,
		inputs:      inputs,
		outputs:     outputs,
		metadata:    make(map[string]interface{}),
	}

	logging.Info("Loaded ONNX model", "model_id", key, "path", m.Path, "inputs", len(inputNames), "outputs", len(outputNames))
	return nil
}

// LoadModelWithTracing loads an ONNX model and records tracing/metrics
func (e *ONNXRuntimeEngine) LoadModelWithTracing(m *model.Model, tracer trace.Tracer) error {
	_, err := e.BaseEngine.modelManager.LoadModel(m.ID, m.Version, m.Path, m.InputShape, m.OutputShape, m.Metadata, tracer)
	if err != nil {
		return err
	}
	return e.LoadModel(m)
}

// UnloadModelWithTracing unloads an ONNX model and records tracing/metrics
func (e *ONNXRuntimeEngine) UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error {
	err := e.BaseEngine.modelManager.UnloadModel(modelID, version, tracer)
	if err != nil {
		return err
	}
	return e.UnloadModel(modelID, version)
}

// Predict performs inference using the ONNX model (now versioned)
func (e *ONNXRuntimeEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error) {
	startTime := time.Now()

	e.mu.RLock()
	key := modelID
	if version != "" {
		key = modelID + ":" + version
	}
	model, exists := e.models[key]
	e.mu.RUnlock()

	if !exists || model.session == nil {
		return nil, fmt.Errorf("model not found or not properly loaded: %s", key)
	}

	if len(model.inputDims) == 0 || len(model.inputNames) == 0 {
		return nil, fmt.Errorf("no input dimensions or names defined for model: %s", modelID)
	}

	// Only support single input for now
	inputData, err := e.convertInput(input, model.inputDims[0])
	if err != nil {
		return nil, fmt.Errorf("failed to convert input: %w", err)
	}
	// Set input tensor data
	inTensor, ok := model.inputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("input tensor is not float32")
	}
	copy(inTensor.GetData(), inputData)

	// Run inference
	if err := model.session.Run(); err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Collect output data
	outTensor, ok := model.outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("output tensor is not float32")
	}
	outputData := outTensor.GetData()
	outputShape := outTensor.GetShape()

	latency := time.Since(startTime).Milliseconds()

	return &Prediction{
		ModelID: modelID,
		Output: map[string]interface{}{
			"data":     outputData,
			"shape":    outputShape,
			"model_id": modelID,
		},
		Metadata: map[string]interface{}{
			"engine":      "onnxruntime",
			"latency_ms":  latency,
			"input_dims":  model.inputDims,
			"output_dims": model.outputDims,
		},
		Latency: latency,
	}, nil
}

// convertInput converts the input data to a format suitable for ONNX Runtime
func (e *ONNXRuntimeEngine) convertInput(input interface{}, dims []int64) ([]float32, error) {
	switch v := input.(type) {
	case []float32:
		return v, nil
	case [][]float32:
		// Flatten 2D slice to 1D
		var flat []float32
		for _, row := range v {
			flat = append(flat, row...)
		}
		return flat, nil
	case []float64:
		// Convert []float64 to []float32
		result := make([]float32, len(v))
		for i, val := range v {
			result[i] = float32(val)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported input type: %T, expected []float32 or [][]float32", input)
	}
}

// BatchPredict performs batch inference using the ONNX model (now versioned)
func (e *ONNXRuntimeEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	var predictions []*Prediction

	for i, input := range inputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			pred, err := e.Predict(ctx, modelID, version, input)
			if err != nil {
				return nil, fmt.Errorf("batch prediction failed at index %d: %w", i, err)
			}
			predictions = append(predictions, pred)
		}
	}

	return predictions, nil
}

// GetModelInfo returns information about a loaded model (now versioned)
func (e *ONNXRuntimeEngine) GetModelInfo(modelID, version string) (*model.Model, error) {
	e.mu.RLock()
	key := modelID
	if version != "" {
		key = modelID + ":" + version
	}
	m, exists := e.models[key]
	e.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("model not found: %s", key)
	}
	// Return a shallow copy or pointer as needed
	return &model.Model{
		ID:       m.modelID,
		Version:  version,
		Path:     m.modelPath,
		Metadata: m.metadata,
	}, nil
}

// ListModels returns a list of loaded model IDs
func (e *ONNXRuntimeEngine) ListModels() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.models))
	for id := range e.models {
		ids = append(ids, id)
	}
	return ids
}

// Ensure ONNXRuntimeEngine implements the Engine interface
var _ Engine = (*ONNXRuntimeEngine)(nil)
