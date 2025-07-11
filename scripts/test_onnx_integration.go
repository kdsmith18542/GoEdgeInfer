//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
)

func main() {
	// Initialize the ONNX Runtime engine
	onnxEngine, err := inference.NewONNXRuntimeEngine()
	if err != nil {
		log.Fatalf("Failed to create ONNX Runtime engine: %v", err)
	}
	defer onnxEngine.Close()

	// Get the absolute path to the test model
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filepath.Dir(filename))
	modelPath := filepath.Join(dir, "testdata", "test_model.onnx")

	// Create a test model
	testModel := &model.Model{
		ID:   "test-model",
		Path: modelPath,
	}

	// Load the model
	err = onnxEngine.LoadModel(testModel)
	if err != nil {
		log.Fatalf("Failed to load model: %v", err)
	}

	// Create test input data matching our test model's expected input shape [1, 3, 32, 32]
	inputDims := []int64{1, 3, 32, 32}
	inputData := make([]float32, inputDims[0]*inputDims[1]*inputDims[2]*inputDims[3])

	// Fill with some test data (normalized values between 0 and 1)
	for i := range inputData {
		inputData[i] = float32(i%255) / 255.0
	}

	fmt.Printf("Running inference with input shape: %v\n", inputDims)

	// Run inference
	result, err := onnxEngine.Predict(context.Background(), testModel.ID, inputData)
	if err != nil {
		log.Fatalf("Inference failed: %v", err)
	}

	// Print the results
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal results: %v", err)
	}

	fmt.Println("Inference results:")
	fmt.Println(string(jsonResult))

	// List all loaded models
	models := onnxEngine.ListModels()
	fmt.Println("\nLoaded models:", models)

	// Get model info
	modelInfo, err := onnxEngine.GetModelInfo(testModel.ID)
	if err != nil {
		log.Fatalf("Failed to get model info: %v", err)
	}

	jsonModelInfo, err := json.MarshalIndent(modelInfo, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal model info: %v", err)
	}

	fmt.Println("\nModel info:")
	fmt.Println(string(jsonModelInfo))
}
