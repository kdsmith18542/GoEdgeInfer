# GoEdgeInfer Remote Registry Management API

## Delete Remote Model
- **POST** `/delete_remote_model`
- **Body:**
  ```json
  { "object_key": "model1.onnx" }
  ```
- **Response:**
  ```json
  { "status": "remote_model_deleted" }
  ```

## Upload Remote Model
- **POST** `/upload_remote_model`
- **Body:**
  ```json
  { "local_path": "/tmp/model1.onnx", "object_key": "model1.onnx" }
  ```
- **Response:**
  ```json
  { "status": "remote_model_uploaded" }
  ```

- All endpoints require API key authentication and are rate-limited.
- Use with caution: deleting from remote registry is irreversible.

---
See `cache_and_registry_api.md` for listing and cache management endpoints.
