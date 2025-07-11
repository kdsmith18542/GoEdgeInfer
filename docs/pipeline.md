# GoEdgeInfer Pipeline Configuration and Usage

## Pipeline Section in config.yaml

The `pipeline` section allows you to define a sequence of pre- and post-processing steps for inference. Each step has a `type` and optional `params`.

### Example
```yaml
pipeline:
  - type: "normalize"
    params: {}
  - type: "softmax"
    params: {}
  - type: "apply_threshold"
    params:
      threshold: 0.7
  - type: "map_to_labels"
    params:
      labels: ["cat", "dog", "bird"]
```

### Supported Steps
- `normalize`: Normalizes a float32 array to [0,1].
- `softmax`: Applies softmax to a float32 array.
- `apply_threshold`: Converts a float32 array to binary (0/1) using a threshold (default 0.5).
- `map_to_labels`: Maps integer array to string labels. Requires `labels` param as a list of strings.

### Usage
- The pipeline is applied to inference input (pre-processing) and output (post-processing).
- You can chain multiple steps as needed.

### Extending
To add custom steps, extend `internal/processing/pipeline.go` with new case statements in the `Run` method.
