package main

import (
	"log/slog"
	"os"

	"github.com/octieght18/forge/internal/execution"
	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	address := os.Getenv("TEMPORAL_ADDRESS")
	if address == "" {
		address = "127.0.0.1:7233"
	}
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	service, err := client.Dial(client.Options{HostPort: address, Namespace: namespace, Logger: slogAdapter{logger}})
	if err != nil {
		logger.Error("temporal client failed")
		os.Exit(1)
	}
	defer service.Close()
	if os.Getenv("FORGE_DEBUG") == "true" {
		execution.UseDebugFixtures()
	} else {
		if command := os.Getenv("FORGE_MCP_COMMAND"); command != "" {
			execution.Corpus = mcp.Stdio{Command: command}
		}
		if key := os.Getenv("FORGE_MODEL_API_KEY"); key != "" {
			execution.Model = model.OpenRouter{Key: key}
		} else if os.Getenv("FORGE_MODEL_FIXTURE") == "true" {
			execution.Model = model.Fixture{}
		}
	}
	process := worker.New(service, execution.TaskQueue, worker.Options{})
	process.RegisterWorkflowWithOptions(execution.Research, workflow.RegisterOptions{Name: execution.WorkflowName})
	process.RegisterActivity(execution.Retrieve)
	process.RegisterActivity(execution.Generate)
	process.RegisterActivity(execution.Compensate)
	process.RegisterActivity(execution.RecordTerminal)
	if err = process.Run(worker.InterruptCh()); err != nil {
		logger.Error("temporal worker stopped")
		os.Exit(1)
	}
}

type slogAdapter struct{ logger *slog.Logger }

func (s slogAdapter) Debug(msg string, keys ...any) { s.logger.Debug(msg, keys...) }
func (s slogAdapter) Info(msg string, keys ...any)  { s.logger.Info(msg, keys...) }
func (s slogAdapter) Warn(msg string, keys ...any)  { s.logger.Warn(msg, keys...) }
func (s slogAdapter) Error(msg string, keys ...any) { s.logger.Error(msg, keys...) }
