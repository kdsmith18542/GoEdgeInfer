package model

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/keith/goedgeinfer/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// DownloadModelFromS3 downloads a model file from S3/MinIO to a local path
func DownloadModelFromS3(ctx context.Context, endpoint, bucket, accessKey, secretKey, region string, useSSL bool, objectKey, destPath string) error {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return err
	}
	obj, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, obj)
	return err
}

// ListRemoteModelsS3 lists all model objects in the configured S3/MinIO bucket/prefix
func ListRemoteModelsS3(ctx context.Context, cfg *config.Config) ([]string, error) {
	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		Secure: cfg.S3.UseSSL,
		Region: cfg.S3.Region,
	})
	if err != nil {
		return nil, err
	}
	bucket := cfg.S3.Bucket
	prefix := ""
	var models []string
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		models = append(models, obj.Key)
	}
	return models, nil
}

// CleanupLocalModelCache removes model files in the given directory not in use
func CleanupLocalModelCache(cacheDir string, keepPaths map[string]struct{}) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(cacheDir, entry.Name())
		if _, keep := keepPaths[fullPath]; !keep {
			os.Remove(fullPath)
		}
	}
	return nil
}

// DeleteModelFromS3 deletes a model object from S3/MinIO
func DeleteModelFromS3(ctx context.Context, cfg *config.Config, objectKey string) error {
	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		Secure: cfg.S3.UseSSL,
		Region: cfg.S3.Region,
	})
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, cfg.S3.Bucket, objectKey, minio.RemoveObjectOptions{})
}

// UploadModelToS3 uploads a local file to S3/MinIO
func UploadModelToS3(ctx context.Context, cfg *config.Config, localPath, objectKey string) error {
	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		Secure: cfg.S3.UseSSL,
		Region: cfg.S3.Region,
	})
	if err != nil {
		return err
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, cfg.S3.Bucket, objectKey, f, stat.Size(), minio.PutObjectOptions{})
	return err
}
