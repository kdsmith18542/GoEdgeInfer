//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/model"
)

func main() {
	// Create a new TFLite engine
	engine := inference.NewTFLiteEngine()
	defer engine.Close()

	// Create a test model
	testModel := &model.Model{
		ID:   "test-model",
		Path: "/path/to/mock/model.tflite",
	}

	// Load the model
	if err := engine.LoadModel(testModel); err != nil {
		log.Fatalf("Failed to load model: %v", err)
	}

	// Create a test input (empty image for mock)
	input := make([]float32, 224*224*3) // Mock input size

	// Make a prediction
	result, err := engine.Predict(context.Background(), "test-model", input)
	if err != nil {
		log.Fatalf("Prediction failed: %v", err)
	}

	// Print the results
	fmt.Println("Mock TFLite Engine Test Results:")
	fmt.Printf("Model ID: %s\n", result.ModelID)
	fmt.Printf("Latency: %d ms\n", result.Latency)
	fmt.Println("Predictions:")

	// Type assert the output to []map[string]interface{}
	if predictions, ok := result.Output.([]map[string]interface{}); ok {
		for i, pred := range predictions {
			label := pred["label"].(string)
			score := pred["score"].(float64)
			fmt.Printf("  %d. %s: %.2f\n", i+1, label, score)
		}
	} else {
		fmt.Printf("Unexpected output type: %T\n", result.Output)
	}

	fmt.Println("\nTest completed successfully!")
}
