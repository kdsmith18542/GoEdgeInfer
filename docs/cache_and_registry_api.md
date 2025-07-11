# GoEdgeInfer Model Cache & Remote Registry API

## Remote Registry

### List Remote Models
- **GET** `/remote_models`
- **Response:**
  ```json
  { "remote_models": ["model1.onnx", "model2.onnx", ...] }
  ```

## Local Model Cache

### Cleanup Local Model Cache
- **POST** `/cleanup_cache`
- **Body:**
  ```json
  { "cache_dir": "/tmp", "keep": ["/tmp/model1.onnx", "/tmp/model2.onnx"] }
  ```
- **Response:**
  ```json
  { "status": "cache_cleaned" }
  ```

- Only files not in the `keep` list will be deleted from the cache directory.

## Security
- All endpoints require API key authentication and are rate-limited.

---
See `management_api.md` for model management endpoints.
