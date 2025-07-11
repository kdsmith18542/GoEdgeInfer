package app

import (
	"fmt"
	"math/rand"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/keith/goedgeinfer/internal/config"
)

func TestNewServerWithConfig_BadPipeline(t *testing.T) {
	badCfg := &config.Config{
		Pipeline:       []map[string]interface{}{{"invalid": true}},
		ServerPort:     "0",
		WorkerPoolSize: 1,
		ModelPath:      "testdata/test_model.onnx",
	}
	// Use a temp dir for queue
	tmpDir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("GOEDGEINFER_QUEUE_PATH", tmpDir)
	defer os.Unsetenv("GOEDGEINFER_QUEUE_PATH")

	_, err = NewServerWithConfig(badCfg)
	if err == nil {
		t.Error("expected error for bad pipeline config, got nil")
	}
}

func TestNewServer_Ok(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("GOEDGEINFER_QUEUE_PATH", tmpDir)
	defer os.Unsetenv("GOEDGEINFER_QUEUE_PATH")

	cfg := &config.Config{
		Pipeline:       []map[string]interface{}{},
		ServerPort:     "0",
		WorkerPoolSize: 1,
		ModelPath:      "testdata/test_model.onnx",
	}
	_, err = NewServerWithConfig(cfg)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func randomPort() string {
	return fmt.Sprintf(":%d", 20000+rand.Intn(10000))
}

func TestNewServerWithConfig_TLSConfigError(t *testing.T) {
	cfg := &config.Config{
		Pipeline:       []map[string]interface{}{},
		ServerPort:     "0",
		WorkerPoolSize: 1,
		ModelPath:      "testdata/test_model.onnx",
		TLS: config.TLSConfig{
			Enabled:           true,
			CertFile:          "nonexistent.crt",
			KeyFile:           "nonexistent.key",
			RequireClientCert: true,
			ClientCA:          "nonexistent_ca.pem",
		},
	}
	tmpDir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("GOEDGEINFER_QUEUE_PATH", tmpDir)
	defer os.Unsetenv("GOEDGEINFER_QUEUE_PATH")
	os.Setenv("GOEDGEINFER_GRPC_ADDR", randomPort())
	defer os.Unsetenv("GOEDGEINFER_GRPC_ADDR")

	_, err = NewServerWithConfig(cfg)
	if err == nil {
		t.Error("expected error for bad TLS config, got nil")
	}
}

func TestNewServerWithConfig_GRPCError(t *testing.T) {
	cfg := &config.Config{
		Pipeline:       []map[string]interface{}{},
		ServerPort:     "0",
		WorkerPoolSize: 1,
		ModelPath:      "testdata/test_model.onnx",
	}
	os.Setenv("GOEDGEINFER_GRPC_ADDR", "invalid:port")
	defer os.Unsetenv("GOEDGEINFER_GRPC_ADDR")
	tmpDir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("GOEDGEINFER_QUEUE_PATH", tmpDir)
	defer os.Unsetenv("GOEDGEINFER_QUEUE_PATH")

	_, _ = NewServerWithConfig(cfg)
}

func TestServer_ReloadAndShutdown(t *testing.T) {
	cfg := &config.Config{
		Pipeline:       []map[string]interface{}{},
		ServerPort:     "0",
		WorkerPoolSize: 1,
		ModelPath:      "testdata/test_model.onnx",
	}
	tmpDir, err := os.MkdirTemp("", "badger-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.Setenv("GOEDGEINFER_QUEUE_PATH", tmpDir)
	defer os.Unsetenv("GOEDGEINFER_QUEUE_PATH")
	os.Setenv("GOEDGEINFER_GRPC_ADDR", randomPort())
	defer os.Unsetenv("GOEDGEINFER_GRPC_ADDR")

	srv, err := NewServerWithConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Simulate reload
	go func() {
		time.Sleep(100 * time.Millisecond)
		srv.ReloadCh <- struct{}{}
		time.Sleep(100 * time.Millisecond)
		srv.ShutdownCh <- syscall.SIGTERM
	}()
	srv.Start()
}
