package execution

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"

	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

func TestDurableRecoveryMeasurements(t *testing.T) {
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
		ResetTerminals()
	})

	half, halfAttempts, halfEvidence := measureHalf(t)
	uncertain, uncertainAttempts := measureUncertain(t)
	canceled, cancelAttempts := measureCancel(t)
	replay := measureReplay(t)

	if halfAttempts["Retrieve"] != 2 || halfAttempts["Generate"] != 1 || halfAttempts["Compensate"] != 1 || halfAttempts["RecordTerminal"] != 1 {
		t.Fatalf("half attempts %#v", halfAttempts)
	}
	if halfEvidence != 1 {
		t.Fatal(halfEvidence)
	}
	if uncertainAttempts["Retrieve"] != 1 || uncertainAttempts["Generate"] != 1 || uncertainAttempts["Compensate"] != 0 || uncertainAttempts["RecordTerminal"] != 1 {
		t.Fatalf("uncertain attempts %#v", uncertainAttempts)
	}
	if cancelAttempts["Retrieve"] < 1 || canceled == "" {
		t.Fatalf("cancel attempts %#v state %s", cancelAttempts, canceled)
	}
	if half <= 0 || uncertain <= 0 || replay < 0 {
		t.Fatal(half, uncertain, replay)
	}
	t.Logf("MEASURE half_us=%d uncertain_us=%d cancel_us=%d replay_us=%d retrieve_attempts=%d generate_attempts=%d compensate_attempts=%d evidence=%d cancel_state=%s",
		half.Microseconds(), uncertain.Microseconds(), cancelObserved.Microseconds(), replay.Microseconds(),
		halfAttempts["Retrieve"], halfAttempts["Generate"], halfAttempts["Compensate"], halfEvidence, canceled)
	_ = cancelAttempts
}

func measureHalf(t *testing.T) (time.Duration, map[string]int, int) {
	t.Helper()
	resetEval()
	tool := &flakyTool{}
	SetCorpus(tool)
	SetModel(nil)
	in := evalInput("abababab-abab-4bab-8bab-abababababab", []string{"platform-brief"}, strings.Repeat("a", 64))
	attempts := map[string]int{}
	started := time.Now()
	runTracked(t, in, attempts)
	elapsed := time.Since(started)
	if tool.calls != 2 {
		t.Fatalf("tool calls %d", tool.calls)
	}
	records := mcp.EvidenceRecords()
	if len(records) != 1 || records[0].Retention != "compensated" {
		t.Fatalf("%#v", records)
	}
	return elapsed, attempts, len(records)
}

func measureUncertain(t *testing.T) (time.Duration, map[string]int) {
	t.Helper()
	resetEval()
	UseDebugFixtures()
	in := evalInput("bcbcbcbc-bcbc-4cbc-8cbc-bcbcbcbcbcbc", []string{"architecture-notes"}, strings.Repeat("b", 64))
	attempts := map[string]int{}
	started := time.Now()
	got := runTracked(t, in, attempts)
	if got.Outcome != "insufficient_evidence" || len(mcp.EvidenceRecords()) != 0 {
		t.Fatalf("%#v records=%d", got, len(mcp.EvidenceRecords()))
	}
	return time.Since(started), attempts
}

func measureCancel(t *testing.T) (string, map[string]int) {
	t.Helper()
	resetEval()
	SetCorpus(blockingTool{})
	SetModel(model.Fixture{})
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	attempts := map[string]int{}
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		attempts[info.ActivityType.Name]++
	})
	env.RegisterActivity(Retrieve)
	env.RegisterActivity(Generate)
	env.RegisterActivity(Compensate)
	env.RegisterActivity(RecordTerminal)
	env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, time.Millisecond)
	in := evalInput("cdcdcdcd-cdcd-4dcd-8dcd-cdcdcdcdcdcd", []string{"platform-brief"}, strings.Repeat("c", 64))
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: in.WorkflowID, WorkflowExecutionTimeout: WorkflowTimeout})
	started := time.Now()
	env.ExecuteWorkflow(Research, in)
	cancelObserved = time.Since(started)
	if !env.IsWorkflowCompleted() {
		t.Fatal("cancel did not finish")
	}
	states := Terminals()
	if len(states) != 1 || states[0].State != "canceled" {
		t.Fatalf("%#v", states)
	}
	return states[0].State, attempts
}

var cancelObserved time.Duration

func measureReplay(t *testing.T) time.Duration {
	t.Helper()
	ResetActivityEntries()
	in := evalInput("33333333-3333-4333-8333-333333333333", []string{"platform-brief"}, strings.Repeat("a", 64))
	started := time.Now()
	if err := Replay(successHistory(t, in), in.WorkflowID); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if ActivityEntries() != 0 {
		t.Fatal(ActivityEntries())
	}
	return elapsed
}

func runTracked(t *testing.T, in Correlation, attempts map[string]int) Result {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		attempts[info.ActivityType.Name]++
		if info.Attempt < 1 {
			t.Fatalf("attempt %d", info.Attempt)
		}
	})
	env.RegisterActivity(Retrieve)
	env.RegisterActivity(Generate)
	env.RegisterActivity(Compensate)
	env.RegisterActivity(RecordTerminal)
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: in.WorkflowID})
	env.ExecuteWorkflow(Research, in)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil {
		t.Fatal(env.GetWorkflowError())
	}
	var got Result
	if err := env.GetWorkflowResult(&got); err != nil {
		t.Fatal(err)
	}
	return got
}
