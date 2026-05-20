# Stage 1: Builder
# Pascal architecture GPUs (GTX 1060/1070/1080, compute capability 6.0/6.1) are supported
FROM nvidia/cuda:12.6.3-devel-ubuntu22.04 AS builder

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install build essentials and protobuf compiler
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    protobuf-compiler \
    wget \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23.11
RUN wget -q https://go.dev/dl/go1.23.11.linux-amd64.tar.gz -O /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Install ONNX Runtime v1.22.0 with GPU support
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-linux-x64-gpu-1.22.0.tgz -O /tmp/onnxruntime.tgz \
    && mkdir -p /opt/onnxruntime \
    && tar -xzf /tmp/onnxruntime.tgz -C /opt/onnxruntime --strip-components=1 \
    && rm /tmp/onnxruntime.tgz
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:${LD_LIBRARY_PATH}"
ENV LIBRARY_PATH="/opt/onnxruntime/lib:${LIBRARY_PATH}"
ENV C_INCLUDE_PATH="/opt/onnxruntime/include:${C_INCLUDE_PATH}"
ENV CPLUS_INCLUDE_PATH="/opt/onnxruntime/include:${CPLUS_INCLUDE_PATH}"

# Target Pascal GPU architecture (compute capability 6.0/6.1)
ENV CUDA_ARCHITECTURES="60;61"

# Enable CGO
ENV CGO_ENABLED=1

# Set working directory
WORKDIR /src

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application binaries
RUN go build -o /app/goedgeinfer cmd/main.go
RUN go build -o /app/goedgeinferctl cmd/goedgeinferctl/main.go

# Stage 2: Runtime
FROM nvidia/cuda:12.6.3-runtime-ubuntu22.04

LABEL gpu.architecture="pascal" \
      gpu.compute_capability="6.0,6.1"

ENV DEBIAN_FRONTEND=noninteractive

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy ONNX Runtime shared libraries from builder
COPY --from=builder /opt/onnxruntime/lib /opt/onnxruntime/lib

# Set library path for ONNX Runtime
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:${LD_LIBRARY_PATH}"

# Copy built binaries from builder
COPY --from=builder /app/goedgeinfer /app/goedgeinfer
COPY --from=builder /app/goedgeinferctl /app/goedgeinferctl

# Copy configs directory
COPY configs/ /app/configs/

# Set working directory
WORKDIR /app

# Expose HTTP and gRPC ports
EXPOSE 8080
EXPOSE 50051

# Set NVIDIA runtime environment variables
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

# Set entrypoint
ENTRYPOINT ["/app/goedgeinfer"]
