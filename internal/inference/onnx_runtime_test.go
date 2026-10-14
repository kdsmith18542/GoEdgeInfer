package inference

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"go.opentelemetry.io/otel/trace"
)

func getTestModelPath(t *testing.T) string {
	path, err := filepath.Abs("../../testdata/test_model.onnx")
	if err != nil {
		t.Fatalf("failed to resolve test model path: %v", err)
	}
	return path
}

func TestONNXRuntimeEngine_Basic(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
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
	m2 := &model.Model{ID: "test", Path: getTestModelPath(t)}
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
	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
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
	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
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
	absPath := getTestModelPath(t)
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
	tracer := trace.NewNoopTracerProvider().Tracer("")
	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
	err = engine.LoadModelWithTracing(m, tracer)
	if err != nil {
		t.Fatalf("LoadModelWithTracing failed: %v", err)
	}
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
	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
	_ = engine.LoadModel(m)
	err = engine.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestModelKey_Validation(t *testing.T) {
	cases := []struct {
		id, version string
		wantKey     string
		wantErr     bool
	}{
		{"m", "", "m", false},
		{"m", "v1", "m:v1", false},
		{"", "", "", true},
		{"m:v", "", "", true},
		{"m", "v:2", "", true},
	}
	for _, c := range cases {
		key, err := modelKey(c.id, c.version)
		if c.wantErr {
			if err == nil {
				t.Errorf("modelKey(%q,%q) expected error", c.id, c.version)
			}
			continue
		}
		if err != nil {
			t.Errorf("modelKey(%q,%q) unexpected error: %v", c.id, c.version, err)
			continue
		}
		if key != c.wantKey {
			t.Errorf("modelKey(%q,%q) = %q, want %q", c.id, c.version, key, c.wantKey)
		}
	}
}

func TestONNXRuntimeEngine_RejectsBadKeys(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()
	m := &model.Model{ID: "bad:id", Path: getTestModelPath(t)}
	if err := engine.LoadModel(m); err == nil {
		t.Error("expected error for model ID containing ':'")
	}
	m2 := &model.Model{ID: "ok", Version: "v:1", Path: getTestModelPath(t)}
	if err := engine.LoadModel(m2); err == nil {
		t.Error("expected error for version containing ':'")
	}
}

func TestResolveONNXRuntimeLibPath_EnvWins(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "libonnxruntime.so")
	if err := os.WriteFile(tmp, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORT_LIB_PATH", tmp)
	got, tried := resolveONNXRuntimeLibPath()
	if got != tmp {
		t.Errorf("resolveONNXRuntimeLibPath() = %q, want %q (tried %v)", got, tmp, tried)
	}
}

func TestResolveONNXRuntimeLibPath_MissingReportsTried(t *testing.T) {
	t.Setenv("ORT_LIB_PATH", "/nonexistent/libonnxruntime.so")
	_, tried := resolveONNXRuntimeLibPath()
	if len(tried) == 0 {
		t.Error("expected tried paths to be reported")
	}
}

func TestFlattenInput_Types(t *testing.T) {
	cases := []struct {
		in   interface{}
		want []float64
	}{
		{[]float32{1, 2}, []float64{1, 2}},
		{[][]float32{{1, 2}, {3}}, []float64{1, 2, 3}},
		{[]float64{1.5}, []float64{1.5}},
		{[][]float64{{1, 2}}, []float64{1, 2}},
		{[]int8{-1, 2}, []float64{-1, 2}},
		{[]uint8{255}, []float64{255}},
		{[]int32{7}, []float64{7}},
		{[]int64{9}, []float64{9}},
		{[]bool{true, false}, []float64{1, 0}},
	}
	for _, c := range cases {
		got, err := flattenInput(c.in)
		if err != nil {
			t.Errorf("flattenInput(%T) error: %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("flattenInput(%T) len = %d, want %d", c.in, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("flattenInput(%T)[%d] = %v, want %v", c.in, i, got[i], c.want[i])
			}
		}
	}
	if _, err := flattenInput("nope"); err == nil {
		t.Error("expected error for unsupported input type")
	}
}

// TestONNXRuntimeEngine_ConcurrentPredict is the A1 regression test: many
// goroutines sharing one loaded model must not interleave tensor writes.
func TestONNXRuntimeEngine_ConcurrentPredict(t *testing.T) {
	engine, err := NewONNXRuntimeEngine()
	if err != nil {
		t.Skip("ONNX runtime not available")
	}
	defer engine.Close()

	m := &model.Model{ID: "test", Path: getTestModelPath(t)}
	if err := engine.LoadModel(m); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}

	const nIn = 3072
	numDistinct := 8
	inputs := make([][]float32, numDistinct)
	expected := make([][]float32, numDistinct)
	for i := range inputs {
		in := make([]float32, nIn)
		for j := range in {
			in[j] = float32(i*1000 + j%50)
		}
		inputs[i] = in
		pred, err := engine.Predict(context.Background(), "test", "", in)
		if err != nil {
			t.Fatalf("baseline Predict: %v", err)
		}
		out := pred.Output.(map[string]interface{})["data"].([]float32)
		cp := make([]float32, len(out))
		copy(cp, out)
		expected[i] = cp
	}

	const goroutines = 32
	const perG = 200
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for r := 0; r < perG; r++ {
				idx := (g + r) % numDistinct
				pred, err := engine.Predict(context.Background(), "test", "", inputs[idx])
				if err != nil {
					errs <- err
					return
				}
				out := pred.Output.(map[string]interface{})["data"].([]float32)
				if len(out) != len(expected[idx]) {
					errs <- fmt.Errorf("output length %d != %d", len(out), len(expected[idx]))
					return
				}
				for k := range out {
					if out[k] != expected[idx][k] {
						errs <- fmt.Errorf("output mismatch at %d: got %v want %v (input %d)", k, out[k], expected[idx][k], idx)
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
