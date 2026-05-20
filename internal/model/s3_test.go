package model

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kdsmith18542/GoEdgeInfer/internal/config"
)

// Mock minio client and config for S3 tests would be ideal, but here we just check error paths and local logic.

func TestCleanupLocalModelCache(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "model1.onnx")
	f2 := filepath.Join(dir, "model2.onnx")
	os.WriteFile(f1, []byte("test1"), 0644)
	os.WriteFile(f2, []byte("test2"), 0644)
	keep := map[string]struct{}{f1: {}}
	if err := CleanupLocalModelCache(dir, keep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("expected %s to exist", f1)
	}
	if _, err := os.Stat(f2); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected %s to be deleted", f2)
	}
}

func TestDownloadModelFromS3_Error(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := DownloadModelFromS3(ctx, "badendpoint", "bucket", "key", "secret", "region", false, "object", "/tmp/shouldnotexist")
	if err == nil {
		t.Error("expected error for bad endpoint")
	}
}

func TestListRemoteModelsS3_Error(t *testing.T) {
	cfg := &config.Config{}
	cfg.S3.Endpoint = "badendpoint"
	cfg.S3.Bucket = "bucket"
	cfg.S3.AccessKey = "key"
	cfg.S3.SecretKey = "secret"
	cfg.S3.Region = "region"
	cfg.S3.UseSSL = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := ListRemoteModelsS3(ctx, cfg)
	if err == nil {
		t.Error("expected error for bad endpoint")
	}
}

func TestDeleteModelFromS3_Error(t *testing.T) {
	cfg := &config.Config{}
	cfg.S3.Endpoint = "badendpoint"
	cfg.S3.Bucket = "bucket"
	cfg.S3.AccessKey = "key"
	cfg.S3.SecretKey = "secret"
	cfg.S3.Region = "region"
	cfg.S3.UseSSL = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := DeleteModelFromS3(ctx, cfg, "object")
	if err == nil {
		t.Error("expected error for bad endpoint")
	}
}

func TestUploadModelToS3_Error(t *testing.T) {
	cfg := &config.Config{}
	cfg.S3.Endpoint = "badendpoint"
	cfg.S3.Bucket = "bucket"
	cfg.S3.AccessKey = "key"
	cfg.S3.SecretKey = "secret"
	cfg.S3.Region = "region"
	cfg.S3.UseSSL = false
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := UploadModelToS3(ctx, cfg, "/does/not/exist", "object")
	if err == nil {
		t.Error("expected error for missing file or bad endpoint")
	}
}
