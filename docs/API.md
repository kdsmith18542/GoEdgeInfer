# GoEdgeInfer API Documentation

## Base URL
All API endpoints are relative to the base URL: `http://localhost:8080`

## Authentication
Currently, the API does not require authentication. In a production environment, consider adding API keys or JWT tokens.

## Endpoints

### Health Check
Check if the API is running.

- **URL**: `/health`
- **Method**: `GET`
- **Response**:
  ```json
  {
    "status": "ok"
  }
  ```

### List Loaded Models
Get a list of all loaded models.

- **URL**: `/models`
- **Method**: `GET`
- **Response**:
  ```json
  [
    "model-1",
    "model-2"
  ]
  ```

### Load a Model
Load a new model into the inference engine.

- **URL**: `/models`
- **Method**: `POST`
- **Request Body**:
  ```json
  {
    "model_id": "test-model",
    "model_path": "/path/to/model.tflite"
  }
  ```
- **Response**:
  ```json
  {
    "model_id": "test-model",
    "status": "model_loaded"
  }
  ```

### Unload a Model
Unload a model from the inference engine.

- **URL**: `/models/:model_id`
- **Method**: `DELETE`
- **URL Parameters**:
  - `model_id`: The ID of the model to unload
- **Response**:
  ```json
  {
    "model_id": "test-model",
    "status": "model_unloaded"
  }
  ```

### Make a Prediction
Make a prediction using a loaded model.

- **URL**: `/predict/:model_id`
- **Method**: `POST`
- **URL Parameters**:
  - `model_id`: The ID of the model to use for prediction
- **Request Body**:
  ```json
  {
    "input": [0.1, 0.2, 0.3, 0.4]
  }
  ```
- **Response**:
  ```json
  {
    "model_id": "test-model",
    "output": [
      {
        "label": "mock_class_1",
        "score": 0.95
      },
      {
        "label": "mock_class_2",
        "score": 0.03
      },
      {
        "label": "mock_class_3",
        "score": 0.02
      }
    ],
    "latency": 0,
    "metadata": {
      "framework": "tflite-mock",
      "input_shape": [1, 224, 224, 3],
      "output_shape": [1, 1000],
      "note": "This is a mock implementation. Install TensorFlow Lite for actual inference."
    }
  }
  ```

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request format"
}
```

### 404 Not Found
```json
{
  "error": "Model not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error"
}
```

## Rate Limiting
Rate limiting is not currently implemented but should be considered for production use.

## Versioning
This is version 1.0.0 of the API.
