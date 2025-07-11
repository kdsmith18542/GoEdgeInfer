#!/bin/bash

# Test API endpoints
set -e

echo "Starting API test..."

# Define the API base URL
API_URL="http://localhost:8080"

# Test health check
echo -e "\nTesting health check..."
curl -s -X GET "$API_URL/health" | jq

# Test model loading
echo -e "\nTesting model loading..."
MODEL_ID="test-model"
MODEL_PATH="/path/to/mock/model.tflite"

curl -s -X POST "$API_URL/models" \
  -H "Content-Type: application/json" \
  -d "{\"model_id\":\"$MODEL_ID\",\"model_path\":\"$MODEL_PATH\"}" | jq

# Test listing models
echo -e "\nListing loaded models..."
curl -s -X GET "$API_URL/models" | jq

# Test prediction
echo -e "\nTesting prediction..."
# Create a test input (mock data)
TEST_INPUT='{"input": [0.1, 0.2, 0.3, 0.4]}'

curl -s -X POST "$API_URL/predict/$MODEL_ID" \
  -H "Content-Type: application/json" \
  -d "$TEST_INPUT" | jq

echo -e "\nAPI test completed successfully!"
