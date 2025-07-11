package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("GOEDGEINFER_SERVER_PORT")
	os.Unsetenv("GOEDGEINFER_WORKER_POOL_SIZE")
	cfg := Load()
	if cfg.ServerPort != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.ServerPort)
	}
	if cfg.WorkerPoolSize != 4 {
		t.Errorf("expected default worker pool size 4, got %d", cfg.WorkerPoolSize)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("GOEDGEINFER_SERVER_PORT", "9999")
	os.Setenv("GOEDGEINFER_WORKER_POOL_SIZE", "7")
	cfg := Load()
	if cfg.ServerPort != "9999" {
		t.Errorf("expected env port 9999, got %s", cfg.ServerPort)
	}
	if cfg.WorkerPoolSize != 7 {
		t.Errorf("expected env worker pool size 7, got %d", cfg.WorkerPoolSize)
	}
	os.Unsetenv("GOEDGEINFER_SERVER_PORT")
	os.Unsetenv("GOEDGEINFER_WORKER_POOL_SIZE")
}
