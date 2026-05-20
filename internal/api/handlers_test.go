package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
	"github.com/kdsmith18542/GoEdgeInfer/internal/persistence"
	"github.com/kdsmith18542/GoEdgeInfer/internal/processing"
	"github.com/kdsmith18542/GoEdgeInfer/internal/worker"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type dummyEngine struct{}

func (d *dummyEngine) Predict(ctx context.Context, modelID, version string, input interface{}) (*inference.Prediction, error) {
	return &inference.Prediction{ModelID: modelID, Output: input, Latency: 1}, nil
}
func (d *dummyEngine) LoadModel(m *model.Model) error                                 { return nil }
func (d *dummyEngine) LoadModelWithTracing(m *model.Model, tracer trace.Tracer) error { return nil }
func (d *dummyEngine) GetModel(modelID, version string) (*model.Model, error)         { return nil, nil }
func (d *dummyEngine) UnloadModel(modelID, version string) error                      { return nil }
func (d *dummyEngine) UnloadModelWithTracing(modelID, version string, tracer trace.Tracer) error {
	return nil
}
func (d *dummyEngine) BatchPredict(ctx context.Context, modelID, version string, inputs []interface{}) ([]*inference.Prediction, error) {
	preds := make([]*inference.Prediction, len(inputs))
	for i, input := range inputs {
		preds[i] = &inference.Prediction{ModelID: modelID, Output: input, Latency: 1}
	}
	return preds, nil
}
func (d *dummyEngine) GetModelInfo(modelID, version string) (*model.Model, error) { return nil, nil }
func (d *dummyEngine) ListModels() []string                                       { return []string{"test_model"} }

type mockModelManager struct {
	ListModelsFunc   func() []string
	GetModelInfoFunc func(modelID, version string) (interface{}, error)
	LoadModelFunc    func(ctx context.Context, modelID, version, path string) error
	UnloadModelFunc  func(ctx context.Context, modelID, version string) error
}

func (m *mockModelManager) ListModels() []string {
	if m.ListModelsFunc != nil {
		return m.ListModelsFunc()
	}
	return []string{"test_model"}
}
func (m *mockModelManager) GetModelInfo(modelID, version string) (interface{}, error) {
	if m.GetModelInfoFunc != nil {
		return m.GetModelInfoFunc(modelID, version)
	}
	return nil, nil
}
func (m *mockModelManager) LoadModel(ctx context.Context, modelID, version, path string) error {
	if m.LoadModelFunc != nil {
		return m.LoadModelFunc(ctx, modelID, version, path)
	}
	return nil
}
func (m *mockModelManager) UnloadModel(ctx context.Context, modelID, version string) error {
	if m.UnloadModelFunc != nil {
		return m.UnloadModelFunc(ctx, modelID, version)
	}
	return nil
}

func newTestAPI(engine inference.Engine, workerPool *worker.WorkerPool, pipeline *processing.Pipeline) *API {
	mockMgr := &mockModelManager{}
	apiInstance := NewAPI(engine, workerPool, mockMgr, trace.NewNoopTracerProvider().Tracer("test"), zap.NewNop())
	apiInstance.pipeline = pipeline
	return apiInstance
}

func TestAPI_Infer_PersistentQueue(t *testing.T) {
	os.RemoveAll("./testdata/api_queue")
	queue, err := persistence.NewPersistentQueue("./testdata/api_queue")
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer queue.Close()

	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, queue, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)

	r := gin.Default()
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	// Prepare request
	input := []float32{1, 2, 3}
	reqBody := InferenceRequest{
		Input: input,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/infer/test_model", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Check that the queue is not empty (task was persisted)
	var out map[string]interface{}
	err = queue.Dequeue(&out)
	if err != nil {
		t.Fatalf("expected task in persistent queue, got error: %v", err)
	}
}

func TestAPI_ListModels(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["models"]; !ok {
		t.Fatalf("expected models key in response")
	}
}

func TestAPI_HealthCheck(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestAPI_Predict_MissingAPIKey(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	input := []float32{1, 2, 3}
	reqBody := InferenceRequest{
		Input: input,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/infer/test_model", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 for missing API key, got %d", w.Code)
	}
}

func TestAPI_ReloadEndpoint(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/system/reload", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestAPI_UnloadModelEndpoint(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/models/test_model", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Fatalf("expected 200 OK or 404 Not Found, got %d", w.Code)
	}
}

func TestAPI_Predict_InvalidInput(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/infer/test_model", bytes.NewReader([]byte("notjson")))
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected error for invalid input, got %d", w.Code)
	}
}

func TestAPI_DoubleReload(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/system/reload", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	// Second reload should also succeed (idempotent)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second reload, got %d", w2.Code)
	}
}

func TestAPI_UnloadNonExistentModel(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/models/doesnotexist", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Fatalf("expected 404 Not Found or 200 OK, got %d", w.Code)
	}
}

func TestAPI_Reload_MissingAPIKey(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/system/reload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 for missing API key, got %d", w.Code)
	}
}

func TestAPI_UnloadModel_MissingAPIKey(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/models/test_model", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 for missing API key, got %d", w.Code)
	}
}

func TestAPI_ListModels_MissingAPIKey(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 for missing API key, got %d", w.Code)
	}
}

type mockRemoteModelManager struct {
	ListRemoteModelsFunc       func(ctx context.Context, cfg *config.Config) ([]string, error)
	CleanupLocalModelCacheFunc func(cacheDir string, keepPaths map[string]struct{}) error
	DeleteModelFromS3Func      func(ctx context.Context, cfg *config.Config, objectKey string) error
	UploadModelToS3Func        func(ctx context.Context, cfg *config.Config, localPath, objectKey string) error
}

func (m *mockRemoteModelManager) ListRemoteModels(ctx context.Context, cfg *config.Config) ([]string, error) {
	return m.ListRemoteModelsFunc(ctx, cfg)
}
func (m *mockRemoteModelManager) CleanupLocalModelCache(cacheDir string, keepPaths map[string]struct{}) error {
	return m.CleanupLocalModelCacheFunc(cacheDir, keepPaths)
}
func (m *mockRemoteModelManager) DeleteModelFromS3(ctx context.Context, cfg *config.Config, objectKey string) error {
	return m.DeleteModelFromS3Func(ctx, cfg, objectKey)
}
func (m *mockRemoteModelManager) UploadModelToS3(ctx context.Context, cfg *config.Config, localPath, objectKey string) error {
	return m.UploadModelToS3Func(ctx, cfg, localPath, objectKey)
}

func TestAPI_ListRemoteModels_Error(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	mockMgr := &mockRemoteModelManager{
		ListRemoteModelsFunc: func(ctx context.Context, cfg *config.Config) ([]string, error) {
			return nil, errors.New("fail")
		},
	}
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	apiInstance.remoteManager = mockMgr
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mgmt/remote/models", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAPI_CleanupModelCache_Success(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	mockMgr := &mockRemoteModelManager{
		CleanupLocalModelCacheFunc: func(cacheDir string, keepPaths map[string]struct{}) error { return nil },
	}
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	apiInstance.remoteManager = mockMgr
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())
	body := []byte(`{"cache_dir":"/tmp","keep":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/remote/cleanup", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPI_DeleteRemoteModel_Error(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	mockMgr := &mockRemoteModelManager{
		DeleteModelFromS3Func: func(ctx context.Context, cfg *config.Config, objectKey string) error { return errors.New("fail") },
	}
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	apiInstance.remoteManager = mockMgr
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())
	body := []byte(`{"object_key":"foo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/remote/delete", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAPI_UploadRemoteModel_Error(t *testing.T) {
	r := gin.Default()
	engine := &dummyEngine{}
	pipeline := &processing.Pipeline{}
	workerPool := worker.NewWorkerPool(engine, 1, nil, pipeline, nil)
	mockMgr := &mockRemoteModelManager{
		UploadModelToS3Func: func(ctx context.Context, cfg *config.Config, localPath, objectKey string) error {
			return errors.New("fail")
		},
	}
	apiInstance := newTestAPI(engine, workerPool, pipeline)
	apiInstance.remoteManager = mockMgr
	SetupRoutes(r, apiInstance, "testkey", config.JWTConfig{}, zap.NewNop())
	body := []byte(`{"local_path":"foo","object_key":"bar"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mgmt/remote/upload", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// Add more management/edge case tests as needed for full coverage
