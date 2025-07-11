import os
import numpy as np
import onnx
from onnx import helper, numpy_helper
from onnx import TensorProto

# Create a simple model with one input and one output
input_shape = [1, 3, 32, 32]
output_shape = [1, 10]  # 10-class classification

# Create input and output tensors
X = helper.make_tensor_value_info('input', TensorProto.FLOAT, input_shape)
Y = helper.make_tensor_value_info('output', TensorProto.FLOAT, output_shape)

# Create a simple model that does a matrix multiplication with random weights
node_def = helper.make_node(
    'Gemm',
    inputs=['input'],
    outputs=['output'],
    alpha=1.0,
    beta=1.0,
    transA=0,
    transB=1
)

# Create the graph (model)
graph_def = helper.make_graph(
    [node_def],
    'test-model',
    [X],
    [Y],
    initializer=[
        numpy_helper.from_array(
            np.random.randn(3 * 32 * 32, 10).astype(np.float32),
            name='weight'
        ),
        numpy_helper.from_array(
            np.random.randn(10).astype(np.float32),
            name='bias'
        )
    ]
)

# Create the model and save it
model_def = helper.make_model(
    graph_def,
    producer_name='onnx-test',
    opset_imports=[helper.make_opsetid("", 10)]
)

# Create testdata directory if it doesn't exist
os.makedirs('testdata', exist_ok=True)

# Save the model
onnx.save(model_def, 'testdata/test_model.onnx')
print("Test model saved to testdata/test_model.onnx")
