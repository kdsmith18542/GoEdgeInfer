# GoEdgeInfer CLI (`goedgeinferctl`)

A command-line tool for managing GoEdgeInfer models, cache, and remote registry.

## Usage

```
goedgeinferctl --server <url> --apikey <key> <command> [args...]

# Or set environment variables:
export GOEDGEINFER_SERVER=http://localhost:8080
export GOEDGEINFER_APIKEY=yourkey
goedgeinferctl <command> [args...]
```

## Commands
- `list-models` — List loaded models
- `load-model <model_id> <model_path> [version]` — Load a model
- `unload-model <model_id>` — Unload a model
- `reload` — Reload config and pipeline
- `list-remote` — List models in remote S3/MinIO registry
- `upload-remote <local_path> <object_key>` — Upload a model to remote registry
- `delete-remote <object_key>` — Delete a model from remote registry
- `cleanup-cache <cache_dir> <keep1> [keep2 ...]` — Clean up local cache, keeping only specified files

## Examples

```
goedgeinferctl list-models
goedgeinferctl load-model mnist /tmp/mnist.onnx v1
goedgeinferctl unload-model mnist
goedgeinferctl reload
goedgeinferctl list-remote
goedgeinferctl upload-remote /tmp/mnist.onnx mnist.onnx
goedgeinferctl delete-remote mnist.onnx
goedgeinferctl cleanup-cache /tmp/modelcache /tmp/modelcache/mnist.onnx
```

All commands require a valid API key.
