package main

import (
	"context"
	"os"

	"github.com/kdsmith18542/GoEdgeInfer/internal/app"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
)

// Run is the main entrypoint for the server. It is factored out for testability.
// exitFunc allows tests to override os.Exit.
func Run(exitFunc func(int)) {
	server, err := app.NewServer()
	if err != nil {
		logging.Fatal("Failed to initialize server", "error", err)
		exitFunc(1)
		return
	}
	defer server.ShutdownFunc()
	defer func() {
		if server.Tracer != nil {
			if err := server.Tracer.Shutdown(context.TODO()); err != nil {
				logging.Error("Error shutting down tracer provider", "error", err)
			}
		}
	}()
	server.Start()
}

func main() {
	Run(os.Exit)
}
