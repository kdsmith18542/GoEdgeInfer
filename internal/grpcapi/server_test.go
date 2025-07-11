package grpcapi

import (
	"context"
	"errors"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/internal/worker"
	"github.com/kdsmith18542/GoEdgeInfer/proto"
	"go.opentelemetry.io/otel/trace"
)

func TestGRPCServer_Basic(t *testing.T) {
	// Add gRPC server test logic here
}

// Dummy engine and worker pool for testing
type dummyEngine struct{}

func (d *dummyEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*inference.Prediction, error) {
	if modelID == "fail" {
		return nil, errors.New("predict error")
	}
	return &inference.Prediction{ModelID: modelID, Output: []float32{1, 2, 3}, Latency: 42}, nil
}
func (d *dummyEngine) LoadModel(m *model.Model) error {
	return nil
}
func (d *dummyEngine) LoadModelWithTracing(m *model.Model, tracer trace.Tracer) error {
	return nil
}
func (d *dummyEngine) GetModel(modelID, version string) (*model.Model, error) {
	return nil, nil
}
func (d *dummyEngine) UnloadModel(modelID, version string) error {
	return nil
}
func (d *dummyEngine) UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error {
	return nil
}
func (d *dummyEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*inference.Prediction, error) {
	return nil, nil
}
func (d *dummyEngine) GetModelInfo(modelID, version string) (*model.Model, error) {
	return nil, nil
}
func (d *dummyEngine) ListModels() []string {
	return []string{"test_model"}
}

func TestGRPCServer_Infer_Success(t *testing.T) {
	engine := &dummyEngine{}
	pool := worker.NewWorkerPool(engine, 1, nil, nil, nil)
	srv := NewServer(engine, pool)

	req := &proto.InferRequest{ModelId: "test_model", InputFloats: []float32{1, 2, 3}, RequestId: "req1"}
	resp, err := srv.Infer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got: %v", resp.Error)
	}
	if resp.ModelId != "test_model" || len(resp.OutputFloats) != 3 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGRPCServer_Infer_MissingModelID(t *testing.T) {
	engine := &dummyEngine{}
	pool := worker.NewWorkerPool(engine, 1, nil, nil, nil)
	srv := NewServer(engine, pool)

	req := &proto.InferRequest{ModelId: "", InputFloats: []float32{1, 2, 3}}
	resp, err := srv.Infer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error == "" {
		t.Errorf("expected error for missing model_id")
	}
}

func TestGRPCServer_Infer_PredictError(t *testing.T) {
	engine := &dummyEngine{}
	pool := worker.NewWorkerPool(engine, 1, nil, nil, nil)
	srv := NewServer(engine, pool)

	req := &proto.InferRequest{ModelId: "fail", InputFloats: []float32{1, 2, 3}}
	resp, err := srv.Infer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error == "" {
		t.Errorf("expected error for predict failure")
	}
}
