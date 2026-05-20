//go:build tflite

package inference

/*
#cgo LDFLAGS: -ltensorflowlite_c
#include <tensorflow/lite/c/c_api.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
)

// TFLiteModel represents a loaded TensorFlow Lite model with its interpreter and C resources
type TFLiteModel struct {
	interpreter *C.TfLiteInterpreter
	model       *C.TfLiteModel
	options     *C.TfLiteInterpreterOptions
	inputSize   int
	outputSize  int
}

// TFLiteEngine is an implementation of the Engine interface using TensorFlow Lite via CGO
type TFLiteEngine struct {
	*BaseEngine
	models map[string]*TFLiteModel
	mu     sync.RWMutex
}

// NewTFLiteEngine creates a new TFLiteEngine instance
func NewTFLiteEngine() *TFLiteEngine {
	return &TFLiteEngine{
		BaseEngine: NewBaseEngine(),
		models:     make(map[string]*TFLiteModel),
	}
}

// Close releases all resources used by the TFLiteEngine
func (e *TFLiteEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for modelID := range e.models {
		e.destroyModel(modelID)
	}

	e.models = make(map[string]*TFLiteModel)
	return nil
}

// destroyModel destroys the C resources for a model without acquiring a lock.
// Caller must hold the lock.
func (e *TFLiteEngine) destroyModel(modelID string) {
	m, exists := e.models[modelID]
	if !exists {
		return
	}
	if m.interpreter != nil {
		C.TfLiteInterpreterDelete(m.interpreter)
	}
	if m.options != nil {
		C.TfLiteInterpreterOptionsDelete(m.options)
	}
	if m.model != nil {
		C.TfLiteModelDelete(m.model)
	}
	delete(e.models, modelID)
	// Best-effort cleanup in the base engine; ignore errors.
	_ = e.BaseEngine.UnloadModel(modelID, "")
}

// LoadModel loads a TensorFlow Lite model from disk using the C API.
func (e *TFLiteEngine) LoadModel(m *model.Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Register in the base engine first.
	if err := e.BaseEngine.LoadModel(m); err != nil {
		return err
	}

	// Read the model file into memory.
	data, err := os.ReadFile(m.Path)
	if err != nil {
		// Roll back the base engine registration.
		_ = e.BaseEngine.UnloadModel(m.ID, "")
		return fmt.Errorf("tflite: failed to read model file %s: %w", m.Path, err)
	}

	// Create TfLiteModel from the buffer.
	cModel := C.TfLiteModelCreate((*C.char)(unsafe.Pointer(&data[0])), C.size_t(len(data)))
	if cModel == nil {
		_ = e.BaseEngine.UnloadModel(m.ID, "")
		return fmt.Errorf("tflite: TfLiteModelCreate failed for %s", m.Path)
	}

	// Create interpreter options.
	opts := C.TfLiteInterpreterOptionsCreate()
	if opts == nil {
		C.TfLiteModelDelete(cModel)
		_ = e.BaseEngine.UnloadModel(m.ID, "")
		return fmt.Errorf("tflite: TfLiteInterpreterOptionsCreate failed")
	}
	// Use one thread by default; callers can make this configurable later.
	C.TfLiteInterpreterOptionsSetNumThreads(opts, 1)

	// Create the interpreter.
	interp := C.TfLiteInterpreterCreate(cModel, opts)
	if interp == nil {
		C.TfLiteInterpreterOptionsDelete(opts)
		C.TfLiteModelDelete(cModel)
		_ = e.BaseEngine.UnloadModel(m.ID, "")
		return fmt.Errorf("tflite: TfLiteInterpreterCreate failed for %s", m.Path)
	}

	// Allocate tensors.
	if status := C.TfLiteInterpreterAllocateTensors(interp); status != C.kTfLiteOk {
		C.TfLiteInterpreterDelete(interp)
		C.TfLiteInterpreterOptionsDelete(opts)
		C.TfLiteModelDelete(cModel)
		_ = e.BaseEngine.UnloadModel(m.ID, "")
		return fmt.Errorf("tflite: AllocateTensors failed (status %d)", int(status))
	}

	// Determine input/output sizes from the first tensor of each.
	inputCount := int(C.TfLiteInterpreterGetInputTensorCount(interp))
	outputCount := int(C.TfLiteInterpreterGetOutputTensorCount(interp))

	inputSize := 0
	if inputCount > 0 {
		inTensor := C.TfLiteInterpreterGetInputTensor(interp, 0)
		if inTensor != nil {
			inputSize = int(C.TfLiteTensorByteSize(inTensor))
		}
	}

	outputSize := 0
	if outputCount > 0 {
		outTensor := C.TfLiteInterpreterGetOutputTensor(interp, 0)
		if outTensor != nil {
			outputSize = int(C.TfLiteTensorByteSize(outTensor))
		}
	}

	e.models[m.ID] = &TFLiteModel{
		interpreter: interp,
		model:       cModel,
		options:     opts,
		inputSize:   inputSize,
		outputSize:  outputSize,
	}

	logging.Info("TFLite model loaded via C API",
		"model_id", m.ID,
		"path", m.Path,
		"input_byte_size", inputSize,
		"output_byte_size", outputSize,
		"input_tensors", inputCount,
		"output_tensors", outputCount,
	)

	return nil
}

// UnloadModel unloads a model and frees its C resources.
func (e *TFLiteEngine) UnloadModel(modelID, version string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.models[modelID]; !exists {
		return model.ErrModelNotFound
	}
	e.destroyModel(modelID)
	return nil
}

// Predict performs inference using the TFLite C API.
func (e *TFLiteEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*Prediction, error) {
	startTime := time.Now()

	e.mu.RLock()
	tfModel, exists := e.models[modelID]
	e.mu.RUnlock()

	if !exists {
		return nil, model.ErrModelNotFound
	}

	// Convert input to []float32.
	inputData, err := toFloat32Slice(input)
	if err != nil {
		return nil, fmt.Errorf("tflite: unsupported input type: %w", err)
	}

	// Copy input data into the input tensor.
	inTensor := C.TfLiteInterpreterGetInputTensor(tfModel.interpreter, 0)
	if inTensor == nil {
		return nil, fmt.Errorf("tflite: failed to get input tensor")
	}

	inputBytes := len(inputData) * 4 // float32 = 4 bytes
	if inputBytes > tfModel.inputSize {
		return nil, fmt.Errorf("tflite: input data (%d bytes) exceeds tensor capacity (%d bytes)", inputBytes, tfModel.inputSize)
	}

	status := C.TfLiteTensorCopyFromBuffer(inTensor, unsafe.Pointer(&inputData[0]), C.size_t(inputBytes))
	if status != C.kTfLiteOk {
		return nil, fmt.Errorf("tflite: TfLiteTensorCopyFromBuffer failed (status %d)", int(status))
	}

	// Run inference.
	if status := C.TfLiteInterpreterInvoke(tfModel.interpreter); status != C.kTfLiteOk {
		return nil, fmt.Errorf("tflite: Invoke failed (status %d)", int(status))
	}

	// Read the output tensor.
	outTensor := C.TfLiteInterpreterGetOutputTensor(tfModel.interpreter, 0)
	if outTensor == nil {
		return nil, fmt.Errorf("tflite: failed to get output tensor")
	}

	outputLen := tfModel.outputSize / 4 // number of float32 values
	outputData := make([]float32, outputLen)
	status = C.TfLiteTensorCopyToBuffer(outTensor, unsafe.Pointer(&outputData[0]), C.size_t(tfModel.outputSize))
	if status != C.kTfLiteOk {
		return nil, fmt.Errorf("tflite: TfLiteTensorCopyToBuffer failed (status %d)", int(status))
	}

	latency := time.Since(startTime).Milliseconds()

	prediction := &Prediction{
		ModelID: modelID,
		Version: version,
		Output:  outputData,
		Latency: latency,
		Metadata: map[string]interface{}{
			"framework":       "tflite",
			"input_byte_size": tfModel.inputSize,
			"output_count":    outputLen,
		},
	}

	return prediction, nil
}

// BatchPredict performs batch inference by iterating over inputs.
func (e *TFLiteEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*Prediction, error) {
	e.mu.RLock()
	_, exists := e.models[modelID]
	e.mu.RUnlock()

	if !exists {
		return nil, model.ErrModelNotFound
	}

	if len(inputs) == 0 {
		return []*Prediction{}, nil
	}

	results := make([]*Prediction, 0, len(inputs))
	for _, input := range inputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pred, err := e.Predict(ctx, modelID, version, input)
		if err != nil {
			logging.Error("Error in batch prediction", "error", err, "model_id", modelID)
			continue
		}
		results = append(results, pred)
	}

	return results, nil
}

// toFloat32Slice converts various numeric slice types to []float32.
func toFloat32Slice(input interface{}) ([]float32, error) {
	switch v := input.(type) {
	case []float32:
		return v, nil
	case []float64:
		out := make([]float32, len(v))
		for i, val := range v {
			out[i] = float32(val)
		}
		return out, nil
	case [][]float32:
		var flat []float32
		for _, row := range v {
			flat = append(flat, row...)
		}
		return flat, nil
	default:
		return nil, fmt.Errorf("unsupported input type %T; expected []float32, []float64, or [][]float32", input)
	}
}
