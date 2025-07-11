# GoEdgeInfer ONNX Runtime Refactor Plan

This file tracks the required changes to align the ONNX inference engine with the latest github.com/yalue/onnxruntime_go API.

## Key Refactor Steps

1. **Session Creation**
   - Use `ort.NewSessionBuilder()` and its methods to configure and create a session.
   - Replace direct `ort.NewSession[float32](...)` calls.

2. **Tensor Creation**
   - Use `ort.NewTensor` with explicit type and shape.
   - Ensure correct flattening and shape handling for input data.

3. **Inference Run**
   - Use the session's `Run` method with named input/output maps.
   - Map input/output names to tensors explicitly.

4. **Input/Output Names and Shapes**
   - Use new API methods to retrieve input/output names and shapes.

5. **Error Handling and Cleanup**
   - Ensure all resources (sessions, tensors) are properly closed/destroyed.

## References
- [onnxruntime_go README](https://github.com/yalue/onnxruntime)
- [test_onnx_integration.go](../scripts/test_onnx_integration.go)
- [GoEdgeInfer.md](../GoEdgeInfer.md)

## TODO
- [ ] Refactor /internal/inference/onnx_runtime.go to use the new API patterns
- [ ] Test and verify ONNX inference end-to-end
- [ ] Update documentation if needed
