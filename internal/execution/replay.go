package execution

import (
	"sync/atomic"

	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

// Mode names the three ways a step can run again.
type Mode string

const (
	// ModeReplay rebuilds decisions from Temporal history and does not call a tool or model.
	ModeReplay Mode = "replay"
	// ModeRetry is the bounded activity retry. It can call a tool or model again.
	ModeRetry Mode = "retry"
	// ModeReexecution is a new run, a new workflow ID, and a rerun_of link.
	ModeReexecution Mode = "reexecution"
)

var activityEntries atomic.Int64

// ActivityEntries counts activity function entries in this process.
// Replay does not increment it.
func ActivityEntries() int64 { return activityEntries.Load() }

// ResetActivityEntries clears the activity entry count. Tests use it.
func ResetActivityEntries() { activityEntries.Store(0) }

func entered() { activityEntries.Add(1) }

// UseDebugFixtures selects the in-process corpus fixture and the fixture model.
// A debug or replay path uses this instead of a configured MCP command or API key.
func UseDebugFixtures() {
	SetCorpus(mcp.FixtureTool{})
	SetModel(model.Fixture{})
}

// Replay runs workflow code against recorded history. Activities are not executed.
func Replay(history *historypb.History, workflowID string) error {
	replayer, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{})
	if err != nil {
		return err
	}
	replayer.RegisterWorkflowWithOptions(Research, workflow.RegisterOptions{Name: WorkflowName})
	return replayer.ReplayWorkflowHistoryWithOptions(nil, history, worker.ReplayWorkflowHistoryOptions{
		OriginalExecution: workflow.Execution{ID: workflowID},
	})
}
