#!/bin/bash

# Test script for TensorFlow Lite integration

echo "Testing TensorFlow Lite integration..."

# Check if the model directory exists
if [ ! -d "test_models/mnist" ]; then
    echo "Creating test model directory..."
    mkdir -p test_models/mnist
fi

# Check if the model file exists
if [ ! -f "test_models/mnist/model.tflite" ]; then
    echo "Warning: No test model found at test_models/mnist/model.tflite"
    echo "Please place a TensorFlow Lite model file at this location for full testing."
    echo "For now, we'll just test the engine initialization."
fi

# Build the test program
echo "Building test program..."
cd scripts
go build -o test_tflite test_tflite_integration.go

# Run the test program
if [ $? -eq 0 ]; then
    echo "Running tests..."
    ./test_tflite
    rm -f test_tflite
else
    echo "Build failed. Please check for compilation errors."
    exit 1
fi

echo "Test completed."
