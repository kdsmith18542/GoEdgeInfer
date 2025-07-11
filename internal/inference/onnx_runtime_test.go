package inference

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"go.opentelemetry.io/otel/trace"
)

func TestONNXRuntimeEngine_Basic(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	_ = engine.LoadModel(m)
	_, _ = engine.Predict(context.Background(), "test", "", []float32{0, 1, 2})
}

func TestONNXRuntimeEngine_LoadModel_Errors(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "notfound", Path: "does_not_exist.onnx"}
	err = engine.LoadModel(m)
	if err == nil {
		t.Error("expected error for missing model file")
	}
	// Load a real model, then try to load again
	m2 := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	_ = engine.LoadModel(m2)
	err = engine.LoadModel(m2)
	if err == nil {
		t.Error("expected error for already loaded model")
	}
}

func TestONNXRuntimeEngine_Predict_Errors(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	// Predict with missing model
	_, err = engine.Predict(context.Background(), "missing", "", []float32{1, 2})
	if err == nil {
		t.Error("expected error for missing model")
	}
	// Load a real model
	m := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	_ = engine.LoadModel(m)
	// Predict with unsupported input type
	_, err = engine.Predict(context.Background(), "test", "", 123)
	if err == nil {
		t.Error("expected error for unsupported input type")
	}
}

func TestONNXRuntimeEngine_BatchPredict(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	_ = engine.LoadModel(m)
	inputs := []interface{}{[]float32{0, 1, 2}, 123} // second input should fail
	_, err = engine.BatchPredict(context.Background(), "test", "", inputs)
	if err == nil {
		t.Error("expected error for bad input in batch predict")
	}
}

func TestONNXRuntimeEngine_GetModelInfo_ListModels(t *testing.T) {
	cwd, _ := os.Getwd()
	t.Logf("Current working directory: %s", cwd)
	absPath, _ := filepath.Abs("../../testdata/test_model.onnx")
	t.Logf("Absolute path to ONNX model: %s", absPath)
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "test", Path: absPath}
	err = engine.LoadModel(m)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	info, err := engine.GetModelInfo("test", "")
	if err != nil || info == nil {
		t.Errorf("GetModelInfo failed: %v", err)
	}
	_, err = engine.GetModelInfo("missing", "")
	if err == nil {
		t.Error("expected error for missing model info")
	}
	ids := engine.ListModels()
	if len(ids) == 0 || ids[0] != "test" {
		t.Errorf("ListModels got %v", ids)
	}
}

func TestONNXRuntimeEngine_UnloadModelWithTracing(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	err = engine.LoadModel(m)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	tracer := trace.NewNoopTracerProvider().Tracer("")
	err = engine.UnloadModelWithTracing("test", "", tracer)
	if err != nil {
		t.Errorf("UnloadModelWithTracing failed: %v", err)
	}
	err = engine.UnloadModelWithTracing("missing", "", tracer)
	if err == nil {
		t.Error("expected error for missing model in UnloadModelWithTracing")
	}
}

func TestONNXRuntimeEngine_Close(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	m := &model.Model{ID: "test", Path: "testdata/test_model.onnx"}
	_ = engine.LoadModel(m)
	err = engine.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
