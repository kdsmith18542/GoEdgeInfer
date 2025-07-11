# GoEdgeInfer Management API

## Endpoints

All endpoints (except `/metrics` and `/health`) require an `X-API-Key` header.

### List Models
- **GET** `/models`
- **Response:**
  ```json
  { "models": ["model_id1", "model_id2", ...] }
  ```

### Load Model
- **POST** `/models`
- **Body:**
  ```json
  { "model_id": "string", "model_path": "string", "version": "string (optional)" }
  ```
- **Response:**
  ```json
  { "model_id": "string", "version": "string", "status": "model_loaded" }
  ```

### Unload Model
- **DELETE** `/models/:model_id`
- **Response:**
  ```json
  { "status": "model_unloaded" }
  ```

### Reload Config/Models
- **POST** `/reload`
- **Response:**
  ```json
  { "status": "reload_triggered" }
  ```

## Notes
- All endpoints are rate-limited and require API key authentication.
- See `/metrics` for Prometheus metrics and `/health` for health checks.
