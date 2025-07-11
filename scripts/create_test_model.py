import os
import numpy as np
import onnx
from onnx import helper, numpy_helper
from onnx import TensorProto

# Use a 2D input for Gemm: [batch_size, features]
input_shape = [1, 3 * 32 * 32]  # 3072 features (flattened image)
output_shape = [1, 10]  # 10-class classification

# Create input and output tensors
X = helper.make_tensor_value_info('input', TensorProto.FLOAT, input_shape)
Y = helper.make_tensor_value_info('output', TensorProto.FLOAT, output_shape)

# Initializers for weights and bias
W_init = numpy_helper.from_array(
    np.random.randn(3 * 32 * 32, 10).astype(np.float32),
    name='weight'
)
B_init = numpy_helper.from_array(
    np.random.randn(10).astype(np.float32),
    name='bias'
)

# Create the Gemm node with all required inputs
node_def = helper.make_node(
    'Gemm',
    inputs=['input', 'weight', 'bias'],
    outputs=['output'],
    alpha=1.0,
    beta=1.0,
    transA=0,
    transB=0  # No transpose, so output shape is [1, 10]
)

# Create the graph (model)
graph_def = helper.make_graph(
    [node_def],
    'test-model',
    [X],
    [Y],
    initializer=[W_init, B_init]
)

# Create the model and save it
model_def = helper.make_model(
    graph_def,
    producer_name='onnx-test',
    opset_imports=[helper.make_opsetid("", 9)]
)

# Patch the IR version for compatibility
model_def.ir_version = 4  # IR version 4 is compatible with ONNX Runtime 1.6+ and opset 9/10

# Create testdata directory if it doesn't exist
os.makedirs('testdata', exist_ok=True)

# Save the model
onnx.save(model_def, 'testdata/test_model.onnx')
print("Test model saved to testdata/test_model.onnx")
