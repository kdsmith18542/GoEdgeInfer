package worker

import (
	"context"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/internal/persistence"
	"github.com/kdsmith18542/GoEdgeInfer/internal/processing"
	"go.opentelemetry.io/otel/trace"
)

// --- Mocks for WorkerPool tests ---
type mockEngine struct{}

func (m *mockEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*inference.Prediction, error) {
	return &inference.Prediction{ModelID: modelID, Output: "ok"}, nil
}
func (m *mockEngine) LoadModel(_ *model.Model) error { return nil }
func (m *mockEngine) GetModel(_, _ string) (*model.Model, error) {
	return nil, nil
}
func (m *mockEngine) UnloadModel(_, _ string) error { return nil }
func (m *mockEngine) BatchPredict(_ context.Context, _, _ string, _ []interface{}) ([]*inference.Prediction, error) {
	return nil, nil
}
func (m *mockEngine) GetModelInfo(_, _ string) (*model.Model, error) {
	return nil, nil
}
func (m *mockEngine) ListModels() []string { return nil }
func (m *mockEngine) LoadModelWithTracing(_ *model.Model, _ trace.Tracer) error {
	return nil
}
func (m *mockEngine) UnloadModelWithTracing(_, _ string, _ trace.Tracer) error {
	return nil
}

type errorEngine struct{}

func (m *errorEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*inference.Prediction, error) {
	return nil, inference.NewInferenceError("fail")
}
func (m *errorEngine) LoadModel(_ *model.Model) error { return nil }
func (m *errorEngine) GetModel(_, _ string) (*model.Model, error) {
	return nil, nil
}
func (m *errorEngine) UnloadModel(_, _ string) error { return nil }
func (m *errorEngine) BatchPredict(_ context.Context, _, _ string, _ []interface{}) ([]*inference.Prediction, error) {
	return nil, nil
}
func (m *errorEngine) GetModelInfo(_, _ string) (*model.Model, error) {
	return nil, nil
}
func (m *errorEngine) ListModels() []string { return nil }
func (m *errorEngine) LoadModelWithTracing(_ *model.Model, _ trace.Tracer) error {
	return nil
}
func (m *errorEngine) UnloadModelWithTracing(_, _ string, _ trace.Tracer) error {
	return nil
}

type mockPipeline struct {
	processing.Pipeline
	err error
}

func (b *mockPipeline) Run(input interface{}) (interface{}, error) { return nil, b.err }

type errorPipeline struct{}

func (e *errorPipeline) Run(input interface{}) (interface{}, error) {
	return nil, inference.NewInferenceError("pipeline fail")
}

// --- Tests ---
func TestWorkerPool_QueueRecovery(t *testing.T) {
	queue, _ := persistence.NewPersistentQueue("./testdata/worker_queue")
	defer queue.Close()
	engine := &inference.MockEngine{}
	pipeline := &processing.Pipeline{}
	pool := NewWorkerPool(engine, 2, queue, pipeline, nil)
	pool.RecoverFromPersistentQueue()
}

func TestWorkerPool_SubmitAndProcess_Success(t *testing.T) {
	pool := NewWorkerPool(&mockEngine{}, 1, nil, nil, nil)
	resultCh, errCh := pool.Submit("model1", "", "input")
	select {
	case res := <-resultCh:
		if res.Output != "ok" {
			t.Errorf("expected output 'ok', got %v", res.Output)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	}
	pool.Shutdown()
}

func TestWorkerPool_SubmitAndProcess_Error(t *testing.T) {
	pool := NewWorkerPool(&errorEngine{}, 1, nil, nil, nil)
	_, errCh := pool.Submit("model1", "", "input")
	if err := <-errCh; err == nil || err.Error() != "fail" {
		t.Errorf("expected error 'fail', got %v", err)
	}
	pool.Shutdown()
}

func TestWorkerPool_PipelineError(t *testing.T) {
	pool := NewWorkerPool(&mockEngine{}, 1, nil, &errorPipeline{}, nil)
	_, errCh := pool.Submit("model1", "", "input")
	if err := <-errCh; err == nil || err.Error() != "pipeline fail" {
		t.Errorf("expected pipeline error, got %v", err)
	}
	pool.Shutdown()
}

func TestWorkerPool_StatsAndUpdatePipeline(t *testing.T) {
	pool := NewWorkerPool(nil, 1, nil, nil, nil)
	_, cap := pool.Stats()
	if cap != 100 {
		t.Errorf("expected cap 100, got %d", cap)
	}
	pool.UpdatePipeline(nil) // just ensure it doesn't panic
	pool.Shutdown()
}
