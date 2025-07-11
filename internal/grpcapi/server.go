package grpcapi

import (
	"context"

	"github.com/keith/goedgeinfer/internal/inference"
	"github.com/keith/goedgeinfer/internal/worker"
	pb "github.com/keith/goedgeinfer/proto"
)

type Server struct {
	pb.UnimplementedGoEdgeInferServiceServer
	engine     inference.Engine
	workerPool *worker.WorkerPool
}

func NewServer(engine inference.Engine, workerPool *worker.WorkerPool) *Server {
	return &Server{
		engine:     engine,
		workerPool: workerPool,
	}
}

// Unary inference
func (s *Server) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	if req.GetModelId() == "" {
		return &pb.InferResponse{Error: "model_id is required"}, nil
	}
	// Use input_floats for now; extend for bytes as needed
	resultCh, errCh := s.workerPool.Submit(req.GetModelId(), req.GetInputFloats())
	select {
	case result := <-resultCh:
		return &pb.InferResponse{
			ModelId:      result.ModelID,
			OutputFloats: toFloat32Slice(result.Output),
			LatencyMs:    result.Latency,
			RequestId:    req.GetRequestId(),
			Error:        "",
		}, nil
	case err := <-errCh:
		return &pb.InferResponse{Error: err.Error()}, nil
	}
}

// Streaming inference (bidirectional)
func (s *Server) StreamInfer(stream pb.GoEdgeInferService_StreamInferServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return err
		}
		if req.GetModelId() == "" {
			resp := &pb.InferResponse{Error: "model_id is required"}
			if sendErr := stream.Send(resp); sendErr != nil {
				return sendErr
			}
			continue
		}
		resultCh, errCh := s.workerPool.Submit(req.GetModelId(), req.GetInputFloats())
		select {
		case result := <-resultCh:
			resp := &pb.InferResponse{
				ModelId:      result.ModelID,
				OutputFloats: toFloat32Slice(result.Output),
				LatencyMs:    result.Latency,
				RequestId:    req.GetRequestId(),
				Error:        "",
			}
			if sendErr := stream.Send(resp); sendErr != nil {
				return sendErr
			}
		case err := <-errCh:
			resp := &pb.InferResponse{Error: err.Error()}
			if sendErr := stream.Send(resp); sendErr != nil {
				return sendErr
			}
		}
	}
}

// Helper to convert output to []float32
func toFloat32Slice(output interface{}) []float32 {
	if arr, ok := output.([]float32); ok {
		return arr
	}
	return nil
}
