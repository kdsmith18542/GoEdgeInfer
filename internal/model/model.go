package model

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"context"
	"errors"

	"github.com/keith/goedgeinfer/internal/config"
	"github.com/keith/goedgeinfer/pkg/logging"
	"github.com/keith/goedgeinfer/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Model represents a loaded ML model
// Add Version field for versioning
type Model struct {
	ID          string
	Version     string // new: model version
	Path        string
	InputShape  []int
	OutputShape []int
	Metadata    map[string]interface{}
}

// ModelManager handles loading and managing ML models
type ModelManager struct {
	models     map[string]*Model
	mu         sync.RWMutex
	downloader ModelDownloader
}

// ModelDownloader interface for downloading models
type ModelDownloader interface {
	Download(ctx context.Context, endpoint, bucket, accessKey, secretKey, region string, useSSL bool, objectKey, localPath string) error
}

type defaultDownloader struct{}

// Download implements the ModelDownloader interface
func (d *defaultDownloader) Download(ctx context.Context, endpoint, bucket, accessKey, secretKey, region string, useSSL bool, objectKey, localPath string) error {
	return DownloadModelFromS3(ctx, endpoint, bucket, accessKey, secretKey, region, useSSL, objectKey, localPath)
}

// NewModelManager creates a new ModelManager instance
func NewModelManager() *ModelManager {
	return &ModelManager{
		models:     make(map[string]*Model),
		downloader: &defaultDownloader{},
	}
}

// NewModelManagerWithDownloader for testing
func NewModelManagerWithDownloader(d ModelDownloader) *ModelManager {
	return &ModelManager{
		models:     make(map[string]*Model),
		downloader: d,
	}
}

// LoadModel loads a model from the given path, with SHA256 integrity check
// Add version argument
// Accepts optional tracer for observability
func (m *ModelManager) LoadModel(id, version, path string, inputShape, outputShape []int, metadata map[string]interface{}, tracer trace.Tracer) (*Model, error) {
	ctx := context.Background()
	var span trace.Span
	if tracer != nil {
		ctx, span = tracer.Start(ctx, "ModelManager.LoadModel", trace.WithAttributes(
			attribute.String("model_id", id),
			attribute.String("version", version),
			attribute.String("path", path),
		))
		defer span.End()
	}
	start := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	key := id
	if version != "" {
		key = id + ":" + version
	}
	if _, exists := m.models[key]; exists {
		metrics.ModelLoadCounter.WithLabelValues(id, version, "error").Inc()
		if span != nil {
			span.RecordError(ErrModelAlreadyExists)
		}
		return nil, ErrModelAlreadyExists
	}

	// Compute SHA256 checksum
	f, err := os.Open(path)
	if err != nil {
		metrics.ModelLoadCounter.WithLabelValues(id, version, "error").Inc()
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		f.Close()
		metrics.ModelLoadCounter.WithLabelValues(id, version, "error").Inc()
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}
	f.Close()
	checksum := hex.EncodeToString(h.Sum(nil))
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["sha256"] = checksum

	model := &Model{
		ID:          id,
		Version:     version,
		Path:        path,
		InputShape:  inputShape,
		OutputShape: outputShape,
		Metadata:    metadata,
	}

	m.models[key] = model
	logging.Info("Model loaded successfully", "model_id", id, "version", version, "path", path, "sha256", checksum)
	metrics.ModelLoadCounter.WithLabelValues(id, version, "success").Inc()
	metrics.ModelLoadDurationHistogram.WithLabelValues(id, version).Observe(time.Since(start).Seconds())
	if span != nil {
		span.SetAttributes(attribute.String("sha256", checksum))
	}

	// Signature verification (if enabled)
	if cfg, ok := metadata["config"].(*config.Config); ok && cfg.SignatureVerification.Enabled {
		sigPath := path + ".sig"
		pubKeyPath := cfg.SignatureVerification.PublicKeyPem
		if err := verifySignature(path, sigPath, pubKeyPath); err != nil {
			metrics.ModelLoadCounter.WithLabelValues(id, version, "error").Inc()
			if span != nil {
				span.RecordError(err)
			}
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	}
	return model, nil
}

// LoadModelWithS3Support loads a model from the given path, with SHA256 integrity check
// If the path starts with "s3://", download from S3/MinIO first
// Accept tracer and pass to LoadModel
func (m *ModelManager) LoadModelWithS3Support(cfg *config.Config, id, version, path string, inputShape, outputShape []int, metadata map[string]interface{}, tracer trace.Tracer) (*Model, error) {
	if strings.HasPrefix(path, "s3://") {
		// Parse s3://bucket/key or s3://key (use cfg.S3.Bucket)
		bucket := cfg.S3.Bucket
		objectKey := strings.TrimPrefix(path, "s3://")
		if strings.Contains(objectKey, "/") {
			parts := strings.SplitN(objectKey, "/", 2)
			if len(parts) == 2 {
				bucket = parts[0]
				objectKey = parts[1]
			}
		}
		localPath := filepath.Join("/tmp", id+"_"+version+".onnx")
		err := m.downloader.Download(context.Background(), cfg.S3.Endpoint, bucket, cfg.S3.AccessKey, cfg.S3.SecretKey, cfg.S3.Region, cfg.S3.UseSSL, objectKey, localPath)
		if err != nil {
			return nil, err
		}
		path = localPath
	}
	return m.LoadModel(id, version, path, inputShape, outputShape, metadata, tracer)
}

// GetModel retrieves a loaded model by ID and optional version
func (m *ModelManager) GetModel(id, version string) (*Model, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := id
	if version != "" {
		key = id + ":" + version
	}
	model, exists := m.models[key]
	return model, exists
}

// UnloadModel removes a loaded model by ID and optional version
// Accepts optional tracer for observability
func (m *ModelManager) UnloadModel(id, version string, tracer trace.Tracer) error {
	ctx := context.Background()
	var span trace.Span
	if tracer != nil {
		ctx, span = tracer.Start(ctx, "ModelManager.UnloadModel", trace.WithAttributes(
			attribute.String("model_id", id),
			attribute.String("version", version),
		))
		defer span.End()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := id
	if version != "" {
		key = id + ":" + version
	}
	if _, exists := m.models[key]; !exists {
		metrics.ModelUnloadCounter.WithLabelValues(id, version, "error").Inc()
		if span != nil {
			span.RecordError(ErrModelNotFound)
		}
		return ErrModelNotFound
	}
	delete(m.models, key)
	logging.Info("Model unloaded", "model_id", id, "version", version)
	metrics.ModelUnloadCounter.WithLabelValues(id, version, "success").Inc()
	return nil
}

// ListModels returns a list of all loaded models (with versions)
func (m *ModelManager) ListModels() []*Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	models := make([]*Model, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, model)
	}
	return models
}

// AddModel adds a model to the manager (for testing)
func (m *ModelManager) AddModel(model *Model) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := model.ID
	if model.Version != "" {
		key = model.ID + ":" + model.Version
	}
	m.models[key] = model
}

// Error variables
var (
	// ErrModelNotFound is returned when a model is not found
	ErrModelNotFound = NewModelError("model not found")
	// ErrModelAlreadyExists is returned when trying to load a model with an existing ID
	ErrModelAlreadyExists = NewModelError("model with this ID already exists")
)

// ModelError represents an error related to model operations
type ModelError struct {
	msg string
}

// NewModelError creates a new ModelError
func NewModelError(msg string) *ModelError {
	return &ModelError{msg: msg}
}

// Error implements the error interface
func (e *ModelError) Error() string {
	return e.msg
}

// verifySignature verifies a file's SHA256 hash against a signature using a PEM public key
func verifySignature(filePath, sigPath, pubKeyPath string) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(pubKeyBytes)
	if block == nil {
		return errors.New("invalid PEM public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	pubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("not ECDSA public key")
	}
	hash := sha256.Sum256(file)
	if len(sig) != 64 {
		return errors.New("invalid signature length")
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pubKey, hash[:], r, s) {
		return errors.New("signature verification failed")
	}
	return nil
}
