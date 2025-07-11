package model

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
)

func tempModelFile(t *testing.T) string {
	f, err := ioutil.TempFile("", "model-*.onnx")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestModelManager_Versioning(t *testing.T) {
	mgr := NewModelManager()
	path := tempModelFile(t)
	defer os.Remove(path)
	m1, err := mgr.LoadModel("foo", "v1", path, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("LoadModel v1 failed: %v", err)
	}
	m2, err := mgr.LoadModel("foo", "v2", path, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("LoadModel v2 failed: %v", err)
	}
	if m1 == m2 {
		t.Error("Expected different model instances for different versions")
	}
	got1, ok1 := mgr.GetModel("foo", "v1")
	if !ok1 || got1 != m1 {
		t.Errorf("GetModel v1 failed: %v, %v", got1, ok1)
	}
	got2, ok2 := mgr.GetModel("foo", "v2")
	if !ok2 || got2 != m2 {
		t.Errorf("GetModel v2 failed: %v, %v", got2, ok2)
	}
}

func TestModelManager_AddAndGet(t *testing.T) {
	mgr := NewModelManager()
	m := &Model{ID: "bar", Version: "v2"}
	mgr.AddModel(m)
	got, ok := mgr.models["bar:v2"]
	if !ok || got == nil {
		t.Fatal("model not found")
	}
}

func TestModelManager_LoadGetUnload(t *testing.T) {
	mgr := NewModelManager()
	path := tempModelFile(t)
	defer os.Remove(path)
	m, err := mgr.LoadModel("id1", "v1", path, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	got, ok := mgr.GetModel("id1", "v1")
	if !ok || got != m {
		t.Errorf("GetModel failed: %v, %v", got, ok)
	}
	if err := mgr.UnloadModel("id1", "v1", nil); err != nil {
		t.Errorf("UnloadModel failed: %v", err)
	}
	_, ok = mgr.GetModel("id1", "v1")
	if ok {
		t.Error("GetModel should fail after unload")
	}
}

func TestModelManager_DuplicateLoad(t *testing.T) {
	mgr := NewModelManager()
	path := tempModelFile(t)
	defer os.Remove(path)
	_, err := mgr.LoadModel("id2", "v2", path, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	_, err = mgr.LoadModel("id2", "v2", path, nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for duplicate model load")
	}
}

// TestModelManager_SignatureVerificationError tests signature verification error path.
func TestModelManager_SignatureVerificationError(t *testing.T) {
	mgr := NewModelManager()
	path := tempModelFile(t)
	defer os.Remove(path)
	cfg := &config.Config{}
	cfg.SignatureVerification.Enabled = true
	cfg.SignatureVerification.PublicKeyPem = "/tmp/nonexistent.pub"
	meta := map[string]interface{}{"config": cfg}
	_, err := mgr.LoadModel("id4", "v4", path, nil, nil, meta, nil)
	if err == nil || err.Error() == "" {
		t.Error("expected signature verification error")
	}
}

func TestModelManager_LoadModel_FileReadError(t *testing.T) {
	mgr := NewModelManager()
	_, err := mgr.LoadModel("id5", "v5", "/tmp/doesnotexist", nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestModelManager_LoadModelWithS3Support_MockDownloader(t *testing.T) {
	downloader := &mockDownloader{}
	mgr := NewModelManagerWithDownloader(downloader)
	cfg := &config.Config{}
	path := "s3://bucket/key"
	m, err := mgr.LoadModelWithS3Support(cfg, "id7", "v7", path, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("LoadModelWithS3Support failed: %v", err)
	}
	if m == nil || m.ID != "id7" || m.Version != "v7" {
		t.Errorf("unexpected model: %+v", m)
	}
	if !downloader.called {
		t.Error("mockDownloader was not called")
	}
}

type mockDownloader struct{ called bool }

func (m *mockDownloader) Download(ctx context.Context, endpoint, bucket, accessKey, secretKey, region string, useSSL bool, objectKey, localPath string) error {
	m.called = true
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}
