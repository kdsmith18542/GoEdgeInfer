package inference

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
	ort "github.com/yalue/onnxruntime_go"
	"go.opentelemetry.io/otel/trace"
)

// sessionSlot is one exclusive session + input/output tensor set.
// A slot is used by at most one goroutine at a time (checked out from the pool).
type sessionSlot struct {
	session *ort.AdvancedSession
	inputs  []ort.Value
	outputs []ort.Value
}

func (s *sessionSlot) destroy() {
	if s == nil {
		return
	}
	if s.session != nil {
		s.session.Destroy()
		s.session = nil
	}
	for _, t := range s.inputs {
		t.Destroy()
	}
	for _, t := range s.outputs {
		t.Destroy()
	}
	s.inputs = nil
	s.outputs = nil
}

// ONNXModel represents a loaded ONNX model with its metadata and a pool of
// session+tensor sets. Tensors are never shared across goroutines: each
// request checks one set out of the pool and returns it when done.
type ONNXModel struct {
	modelID     string
	modelPath   string
	inputNames  []string
	outputNames []string
	inputDims   [][]int64
	outputDims  [][]int64
	inputTypes  []ort.TensorElementDataType
	outputTypes []ort.TensorElementDataType
	metadata    map[string]interface{}
	pool        chan *sessionSlot
}

// ONNXRuntimeEngine is an implementation of the Engine interface using ONNX Runtime
type ONNXRuntimeEngine struct {
	*BaseEngine
	models map[string]*ONNXModel
	mu     sync.RWMutex
}

// wellKnownONNXRuntimePaths lists shared-library locations probed when neither
// ORT_LIB_PATH nor onnxruntime.lib_path is set.
func wellKnownONNXRuntimePaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/opt/onnxruntime/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so",
		"/usr/lib/libonnxruntime.so",
		"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
		filepath.Join(home, "onnxruntime/onnxruntime-linux-x64-1.22.0/lib/libonnxruntime.so"),
	}
}

// resolveONNXRuntimeLibPath picks the ONNX Runtime shared library:
// ORT_LIB_PATH env, then onnxruntime.lib_path config, then well-known paths.
func resolveONNXRuntimeLibPath() (string, []string) {
	tried := []string{}
	if p := os.Getenv("ORT_LIB_PATH"); p != "" {
		tried = append(tried, p)
		if _, err := os.Stat(p); err == nil {
			return p, tried
		}
	}
	if config.AppConfig != nil && config.AppConfig.ONNXRuntime.LibPath != "" {
		p := config.AppConfig.ONNXRuntime.LibPath
		tried = append(tried, p)
		if _, err := os.Stat(p); err == nil {
			return p, tried
		}
	}
	for _, p := range wellKnownONNXRuntimePaths() {
		tried = append(tried, p)
		if _, err := os.Stat(p); err == nil {
			return p, tried
		}
	}
	return "", tried
}

// sessionPoolSize returns the per-model session count: explicit config value,
// else the worker pool size, else 4.
func sessionPoolSize() int {
	if config.AppConfig != nil {
		if n := config.AppConfig.ONNXRuntime.SessionPoolSize; n > 0 {
			return n
		}
		if n := config.AppConfig.WorkerPoolSize; n > 0 {
			return n
		}
	}
	return 4
}

// modelKey builds the internal registry key and rejects keys that would
// break the id:version encoding.
func modelKey(id, version string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("model ID must not be empty")
	}
	if strings.Contains(id, ":") {
		return "", fmt.Errorf("model ID %q must not contain ':'", id)
	}
	if strings.Contains(version, ":") {
		return "", fmt.Errorf("model version %q must not contain ':'", version)
	}
	if version == "" {
		return id, nil
	}
	return id + ":" + version, nil
}

// NewONNXRuntimeEngine creates a new ONNX Runtime engine instance
func NewONNXRuntimeEngine() (*ONNXRuntimeEngine, error) {
	logging.Info("Initializing ONNX Runtime engine")

	libPath, tried := resolveONNXRuntimeLibPath()
	if libPath == "" {
		return nil, fmt.Errorf("ONNX Runtime shared library not found; set ORT_LIB_PATH or onnxruntime.lib_path (tried: %s)", strings.Join(tried, ", "))
	}
	ort.SetSharedLibraryPath(libPath)

	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX environment (lib %s): %w", libPath, err)
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

	for _, m := range e.models {
		close(m.pool)
		for slot := range m.pool {
			slot.destroy()
		}
	}
	e.models = make(map[string]*ONNXModel)

	logging.Info("ONNX Runtime engine closed")
	return nil
}

func (e *ONNXRuntimeEngine) unloadModel(key string) error {
	if m, exists := e.models[key]; exists {
		close(m.pool)
		for slot := range m.pool {
			slot.destroy()
		}
		delete(e.models, key)
		logging.Info("Unloaded ONNX model", "model_id", key)
	}
	return nil
}

// GetModel retrieves a loaded model by ID and version
func (e *ONNXRuntimeEngine) GetModel(modelID, version string) (*model.Model, error) {
	return e.GetModelInfo(modelID, version)
}

// UnloadModel unloads an ONNX model from memory by ID and version
func (e *ONNXRuntimeEngine) UnloadModel(modelID, version string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key, err := modelKey(modelID, version)
	if err != nil {
		return err
	}
	if _, exists := e.models[key]; !exists {
		return model.ErrModelNotFound
	}
	return e.unloadModel(key)
}

func newInputTensor(dtype ort.TensorElementDataType, dims ort.Shape) (ort.Value, error) {
	n := int(dims.FlattenedSize())
	switch dtype {
	case ort.TensorElementDataTypeFloat:
		return ort.NewTensor[float32](dims, make([]float32, n))
	case ort.TensorElementDataTypeDouble:
		return ort.NewTensor[float64](dims, make([]float64, n))
	case ort.TensorElementDataTypeInt8:
		return ort.NewTensor[int8](dims, make([]int8, n))
	case ort.TensorElementDataTypeUint8:
		return ort.NewTensor[uint8](dims, make([]uint8, n))
	case ort.TensorElementDataTypeInt16:
		return ort.NewTensor[int16](dims, make([]int16, n))
	case ort.TensorElementDataTypeUint16:
		return ort.NewTensor[uint16](dims, make([]uint16, n))
	case ort.TensorElementDataTypeInt32:
		return ort.NewTensor[int32](dims, make([]int32, n))
	case ort.TensorElementDataTypeUint32:
		return ort.NewTensor[uint32](dims, make([]uint32, n))
	case ort.TensorElementDataTypeInt64:
		return ort.NewTensor[int64](dims, make([]int64, n))
	case ort.TensorElementDataTypeUint64:
		return ort.NewTensor[uint64](dims, make([]uint64, n))
	case ort.TensorElementDataTypeBool:
		return ort.NewTensor[bool](dims, make([]bool, n))
	default:
		return nil, fmt.Errorf("unsupported input tensor dtype: %v", dtype)
	}
}

func newOutputTensor(dtype ort.TensorElementDataType, dims ort.Shape) (ort.Value, error) {
	n := int(dims.FlattenedSize())
	switch dtype {
	case ort.TensorElementDataTypeFloat:
		return ort.NewTensor[float32](dims, make([]float32, n))
	case ort.TensorElementDataTypeDouble:
		return ort.NewTensor[float64](dims, make([]float64, n))
	case ort.TensorElementDataTypeInt8:
		return ort.NewTensor[int8](dims, make([]int8, n))
	case ort.TensorElementDataTypeUint8:
		return ort.NewTensor[uint8](dims, make([]uint8, n))
	case ort.TensorElementDataTypeInt16:
		return ort.NewTensor[int16](dims, make([]int16, n))
	case ort.TensorElementDataTypeUint16:
		return ort.NewTensor[uint16](dims, make([]uint16, n))
	case ort.TensorElementDataTypeInt32:
		return ort.NewTensor[int32](dims, make([]int32, n))
	case ort.TensorElementDataTypeUint32:
		return ort.NewTensor[uint32](dims, make([]uint32, n))
	case ort.TensorElementDataTypeInt64:
		return ort.NewTensor[int64](dims, make([]int64, n))
	case ort.TensorElementDataTypeUint64:
		return ort.NewTensor[uint64](dims, make([]uint64, n))
	case ort.TensorElementDataTypeBool:
		return ort.NewTensor[bool](dims, make([]bool, n))
	default:
		return nil, fmt.Errorf("unsupported output tensor dtype: %v", dtype)
	}
}

func buildSessionOpts() (*ort.SessionOptions, error) {
	if config.AppConfig == nil || config.AppConfig.ExecutionProvider.Type != "cuda" {
		return nil, nil
	}
	sessionOpts, err := ort.NewSessionOptions()
	if err != nil {
		logging.Warn("Failed to create session options for CUDA provider, falling back to CPU", "error", err)
		return nil, nil
	}
	cudaOpts, err := ort.NewCUDAProviderOptions()
	if err != nil {
		logging.Warn("Failed to create CUDA provider options, falling back to CPU", "error", err)
		sessionOpts.Destroy()
		return nil, nil
	}
	updateMap := map[string]string{
		"device_id": fmt.Sprintf("%d", config.AppConfig.ExecutionProvider.DeviceID),
	}
	if err := cudaOpts.Update(updateMap); err != nil {
		logging.Warn("Failed to update CUDA provider options, falling back to CPU", "error", err)
		cudaOpts.Destroy()
		sessionOpts.Destroy()
		return nil, nil
	}
	if err := sessionOpts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
		logging.Warn("Failed to append CUDA execution provider, falling back to CPU", "error", err)
		cudaOpts.Destroy()
		sessionOpts.Destroy()
		return nil, nil
	}
	cudaOpts.Destroy()
	logging.Info("Enabled CUDA execution provider", "device_id", config.AppConfig.ExecutionProvider.DeviceID)
	return sessionOpts, nil
}

func createSessionSlot(modelPath string, inputNames, outputNames []string,
	inputTypes, outputTypes []ort.TensorElementDataType,
	inputDims, outputDims [][]int64, sessionOpts *ort.SessionOptions) (*sessionSlot, error) {

	inputs := make([]ort.Value, len(inputNames))
	outputs := make([]ort.Value, len(outputNames))

	for i, dims := range inputDims {
		t, err := newInputTensor(inputTypes[i], ort.Shape(dims))
		if err != nil {
			for _, prev := range inputs {
				if prev != nil {
					prev.Destroy()
				}
			}
			return nil, fmt.Errorf("failed to create input tensor %q: %w", inputNames[i], err)
		}
		inputs[i] = t
	}
	for i, dims := range outputDims {
		t, err := newOutputTensor(outputTypes[i], ort.Shape(dims))
		if err != nil {
			for _, prev := range inputs {
				if prev != nil {
					prev.Destroy()
				}
			}
			for _, prev := range outputs {
				if prev != nil {
					prev.Destroy()
				}
			}
			return nil, fmt.Errorf("failed to create output tensor %q: %w", outputNames[i], err)
		}
		outputs[i] = t
	}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, sessionOpts)
	if err != nil {
		for _, t := range inputs {
			t.Destroy()
		}
		for _, t := range outputs {
			t.Destroy()
		}
		return nil, fmt.Errorf("failed to create ONNX AdvancedSession: %w", err)
	}

	return &sessionSlot{session: session, inputs: inputs, outputs: outputs}, nil
}

// LoadModel loads an ONNX model from the given path
func (e *ONNXRuntimeEngine) LoadModel(m *model.Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key, err := modelKey(m.ID, m.Version)
	if err != nil {
		return err
	}
	if _, exists := e.models[key]; exists {
		return fmt.Errorf("model with ID %s is already loaded", key)
	}

	if _, err := os.Stat(m.Path); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", m.Path)
	}

	inputInfos, outputInfos, err := ort.GetInputOutputInfo(m.Path)
	if err != nil {
		return fmt.Errorf("failed to get input/output info: %w", err)
	}
	if len(inputInfos) == 0 {
		return fmt.Errorf("model %s has no inputs", key)
	}

	inputNames := make([]string, len(inputInfos))
	inputDims := make([][]int64, len(inputInfos))
	inputTypes := make([]ort.TensorElementDataType, len(inputInfos))
	for i, info := range inputInfos {
		inputNames[i] = info.Name
		inputDims[i] = []int64(info.Dimensions)
		inputTypes[i] = info.DataType
	}
	outputNames := make([]string, len(outputInfos))
	outputDims := make([][]int64, len(outputInfos))
	outputTypes := make([]ort.TensorElementDataType, len(outputInfos))
	for i, info := range outputInfos {
		outputNames[i] = info.Name
		outputDims[i] = []int64(info.Dimensions)
		outputTypes[i] = info.DataType
	}

	sessionOpts, err := buildSessionOpts()
	if err != nil {
		return err
	}
	if sessionOpts != nil {
		defer sessionOpts.Destroy()
	}

	n := sessionPoolSize()
	pool := make(chan *sessionSlot, n)
	for i := 0; i < n; i++ {
		slot, err := createSessionSlot(m.Path, inputNames, outputNames, inputTypes, outputTypes, inputDims, outputDims, sessionOpts)
		if err != nil {
			for existing := range pool {
				existing.destroy()
			}
			return err
		}
		pool <- slot
	}

	e.models[key] = &ONNXModel{
		modelID:     m.ID,
		modelPath:   m.Path,
		inputNames:  inputNames,
		outputNames: outputNames,
		inputDims:   inputDims,
		outputDims:  outputDims,
		inputTypes:  inputTypes,
		outputTypes: outputTypes,
		metadata:    make(map[string]interface{}),
		pool:        pool,
	}

	logging.Info("Loaded ONNX model", "model_id", key, "path", m.Path,
		"inputs", len(inputNames), "outputs", len(outputNames), "sessions", n)
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

// flattenInput converts supported input shapes to a flat numeric slice.
func flattenInput(input interface{}) ([]float64, error) {
	switch v := input.(type) {
	case []float32:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out, nil
	case [][]float32:
		var out []float64
		for _, row := range v {
			for _, x := range row {
				out = append(out, float64(x))
			}
		}
		return out, nil
	case []float64:
		return v, nil
	case [][]float64:
		var out []float64
		for _, row := range v {
			out = append(out, row...)
		}
		return out, nil
	case []int8:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out, nil
	case []uint8:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out, nil
	case []int32:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out, nil
	case []int64:
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out, nil
	case []bool:
		out := make([]float64, len(v))
		for i, x := range v {
			if x {
				out[i] = 1
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported input type: %T, expected a numeric slice or map[string]interface{}", input)
	}
}

// fillTensor writes flat numeric data into a tensor, converting to its dtype.
func fillTensor(t ort.Value, flat []float64) error {
	switch tv := t.(type) {
	case *ort.Tensor[float32]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = float32(flat[i])
		}
	case *ort.Tensor[float64]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		copy(d, flat)
	case *ort.Tensor[int8]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = int8(flat[i])
		}
	case *ort.Tensor[uint8]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = uint8(flat[i])
		}
	case *ort.Tensor[int16]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = int16(flat[i])
		}
	case *ort.Tensor[uint16]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = uint16(flat[i])
		}
	case *ort.Tensor[int32]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = int32(flat[i])
		}
	case *ort.Tensor[uint32]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = uint32(flat[i])
		}
	case *ort.Tensor[int64]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = int64(flat[i])
		}
	case *ort.Tensor[uint64]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = uint64(flat[i])
		}
	case *ort.Tensor[bool]:
		d := tv.GetData()
		if len(d) != len(flat) {
			return fmt.Errorf("input size %d does not match tensor size %d", len(flat), len(d))
		}
		for i := range d {
			d[i] = flat[i] != 0
		}
	default:
		return fmt.Errorf("unsupported tensor type %T", t)
	}
	return nil
}

// readTensor copies data out of a tensor (copy so the slot can be reused safely).
func readTensor(t ort.Value) (interface{}, []int64, error) {
	switch tv := t.(type) {
	case *ort.Tensor[float32]:
		src := tv.GetData()
		out := make([]float32, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[float64]:
		src := tv.GetData()
		out := make([]float64, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[int8]:
		src := tv.GetData()
		out := make([]int8, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[uint8]:
		src := tv.GetData()
		out := make([]uint8, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[int16]:
		src := tv.GetData()
		out := make([]int16, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[uint16]:
		src := tv.GetData()
		out := make([]uint16, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[int32]:
		src := tv.GetData()
		out := make([]int32, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[uint32]:
		src := tv.GetData()
		out := make([]uint32, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[int64]:
		src := tv.GetData()
		out := make([]int64, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[uint64]:
		src := tv.GetData()
		out := make([]uint64, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	case *ort.Tensor[bool]:
		src := tv.GetData()
		out := make([]bool, len(src))
		copy(out, src)
		return out, []int64(tv.GetShape()), nil
	default:
		return nil, nil, fmt.Errorf("unsupported tensor type %T", t)
	}
}

// resolveInputs maps the request input onto the model's input tensors.
// A map[string]interface{} is keyed by input name (multi-input models);
// a bare slice is shorthand for single-input models.
func resolveInputs(m *ONNXModel, input interface{}) ([][]float64, error) {
	if named, ok := input.(map[string]interface{}); ok {
		out := make([][]float64, len(m.inputNames))
		for i, name := range m.inputNames {
			raw, ok := named[name]
			if !ok {
				return nil, fmt.Errorf("missing input %q", name)
			}
			flat, err := flattenInput(raw)
			if err != nil {
				return nil, fmt.Errorf("input %q: %w", name, err)
			}
			out[i] = flat
		}
		return out, nil
	}
	if len(m.inputNames) != 1 {
		return nil, fmt.Errorf("model %s has %d inputs; pass map[string]interface{} keyed by input name", m.modelID, len(m.inputNames))
	}
	flat, err := flattenInput(input)
	if err != nil {
		return nil, err
	}
	return [][]float64{flat}, nil
}

// Predict performs inference using the ONNX model (now versioned)
func (e *ONNXRuntimeEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error) {
	startTime := time.Now()

	key, err := modelKey(modelID, version)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	m, exists := e.models[key]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found or not properly loaded: %s", key)
	}

	inputs, err := resolveInputs(m, input)
	if err != nil {
		return nil, err
	}

	select {
	case slot := <-m.pool:
		defer func() { m.pool <- slot }()
		return e.runSlot(m, slot, inputs, modelID, startTime)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *ONNXRuntimeEngine) runSlot(m *ONNXModel, slot *sessionSlot, inputs [][]float64, modelID string, startTime time.Time) (*Prediction, error) {
	for i, flat := range inputs {
		if err := fillTensor(slot.inputs[i], flat); err != nil {
			return nil, fmt.Errorf("failed to fill input %q: %w", m.inputNames[i], err)
		}
	}

	if err := slot.session.Run(); err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	outData := make([]interface{}, len(slot.outputs))
	outShapes := make([][]int64, len(slot.outputs))
	for i, t := range slot.outputs {
		data, shape, err := readTensor(t)
		if err != nil {
			return nil, fmt.Errorf("failed to read output %q: %w", m.outputNames[i], err)
		}
		outData[i] = data
		outShapes[i] = shape
	}

	latency := time.Since(startTime).Milliseconds()

	output := map[string]interface{}{
		"data":     outData[0],
		"shape":    outShapes[0],
		"model_id": modelID,
	}
	if len(outData) > 1 {
		named := make(map[string]interface{}, len(outData))
		for i, name := range m.outputNames {
			named[name] = map[string]interface{}{
				"data":  outData[i],
				"shape": outShapes[i],
			}
		}
		output["outputs"] = named
	}

	return &Prediction{
		ModelID: modelID,
		Output:  output,
		Metadata: map[string]interface{}{
			"engine":      "onnxruntime",
			"latency_ms":  latency,
			"input_dims":  m.inputDims,
			"output_dims": m.outputDims,
		},
		Latency: latency,
	}, nil
}

// PredictSequential runs each input through Predict one at a time.
// There is no throughput gain over calling Predict in a loop: this is
// sequential execution, not fused batching. Real dynamic batching that
// coalesces concurrent requests into one session.Run is plan.md B3.
func (e *ONNXRuntimeEngine) PredictSequential(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	var predictions []*Prediction

	for i, input := range inputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			pred, err := e.Predict(ctx, modelID, version, input)
			if err != nil {
				return nil, fmt.Errorf("sequential prediction failed at index %d: %w", i, err)
			}
			predictions = append(predictions, pred)
		}
	}

	return predictions, nil
}

// BatchPredict satisfies the Engine interface. It delegates to
// PredictSequential: inputs are executed one per session run, NOT fused
// through the ORT batch dimension. Do not rely on it for throughput; use
// concurrent Predict calls (the session pool handles parallelism) or wait
// for plan.md B3 dynamic batching.
func (e *ONNXRuntimeEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	return e.PredictSequential(ctx, modelID, version, inputs)
}

// GetModelInfo returns information about a loaded model (now versioned)
func (e *ONNXRuntimeEngine) GetModelInfo(modelID, version string) (*model.Model, error) {
	key, err := modelKey(modelID, version)
	if err != nil {
		return nil, err
	}
	e.mu.RLock()
	m, exists := e.models[key]
	e.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("model not found: %s", key)
	}
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
