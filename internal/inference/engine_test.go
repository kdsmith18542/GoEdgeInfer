package inference

import (
	"context"
	"os"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"go.opentelemetry.io/otel/trace"
)

func makeTestModel(id, version string) *model.Model {
	return &model.Model{
		ID:      id,
		Version: version,
		Path:    "/tmp/fake",
	}
}

func TestBaseEngine_ModelLifecycle(t *testing.T) {
	eng := NewBaseEngine()
	m := makeTestModel("m1", "v1")
	if err := eng.LoadModel(m); err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	got, err := eng.GetModel("m1", "v1")
	if err != nil || got == nil {
		t.Errorf("GetModel should return model after load, got %v, %v", got, err)
	}
	if err := eng.UnloadModel("m1", "v1"); err != nil {
		t.Errorf("UnloadModel failed: %v", err)
	}
	_, err = eng.GetModel("m1", "v1")
	if err == nil {
		t.Error("GetModel should return error after unload")
	}
}

func TestBaseEngine_Predict_NotImplemented(t *testing.T) {
	eng := NewBaseEngine()
	m := makeTestModel("m2", "v2")
	eng.LoadModel(m)
	_, err := eng.Predict(context.Background(), "m2", "v2", "input")
	if err != ErrNotImplemented {
		t.Errorf("Predict should return ErrNotImplemented, got %v", err)
	}
}

func TestBaseEngine_BatchPredict_NotImplemented(t *testing.T) {
	eng := NewBaseEngine()
	_, err := eng.BatchPredict(context.Background(), "m2", "v2", []interface{}{"input"})
	if err != ErrNotImplemented {
		t.Errorf("BatchPredict should return ErrNotImplemented, got %v", err)
	}
}

func TestBaseEngine_ListModels(t *testing.T) {
	eng := NewBaseEngine()
	m := makeTestModel("m3", "v3")
	eng.LoadModel(m)
	models := eng.ListModels()
	if len(models) != 1 || models[0] != "m3:v3" {
		t.Errorf("ListModels got %v", models)
	}
}

func TestBaseEngine_GetModelInfo(t *testing.T) {
	eng := NewBaseEngine()
	m := makeTestModel("m4", "v4")
	eng.LoadModel(m)
	got, err := eng.GetModelInfo("m4", "v4")
	if err != nil || got == nil {
		t.Errorf("GetModelInfo should return model, got %v, %v", got, err)
	}
	_, err = eng.GetModelInfo("notfound", "v4")
	if err == nil {
		t.Error("GetModelInfo should return error for missing model")
	}
}

func TestBaseEngine_LoadModel_AlreadyExists(t *testing.T) {
	eng := NewBaseEngine()
	m := makeTestModel("m5", "v5")
	if err := eng.LoadModel(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := eng.LoadModel(m); err == nil {
		t.Error("expected error for already existing model, got nil")
	}
}

func TestBaseEngine_LoadUnloadModelWithTracing(t *testing.T) {
	eng := NewBaseEngine()
	// Create a temp file to simulate a real model file
	tmpfile, err := os.CreateTemp("", "model-*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	m := makeTestModel("m6", "v6")
	m.Path = tmpfile.Name()
	tracer := trace.NewNoopTracerProvider().Tracer("")
	if err := eng.LoadModelWithTracing(m, tracer); err != nil {
		t.Errorf("LoadModelWithTracing failed: %v", err)
	}
	if err := eng.UnloadModelWithTracing("m6", "v6", tracer); err != nil {
		t.Errorf("UnloadModelWithTracing failed: %v", err)
	}
}
