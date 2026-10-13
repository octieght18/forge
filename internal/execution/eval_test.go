package execution

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/octieght18/forge/internal/audit"
	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

func TestEvaluationFixtures(t *testing.T) {
	if ModeReplay == ModeRetry || ModeRetry == ModeReexecution || ModeReplay == "" {
		t.Fatal(ModeReplay, ModeRetry, ModeReexecution)
	}
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
		ResetTerminals()
	})
	t.Run("success", func(t *testing.T) {
		resetEval()
		UseDebugFixtures()
		in := evalInput("55555555-5555-4555-8555-555555555555", []string{"platform-brief"}, strings.Repeat("a", 64))
		got := runEval(t, in)
		if got.Decision != "complete" || got.Outcome != "answered" || len(got.EvidenceIDs) != 1 {
			t.Fatalf("%#v", got)
		}
		records := mcp.EvidenceRecords()
		if len(records) != 1 || records[0].Retention != mcp.RetentionWorkflowHistory || records[0].EvidenceID != got.EvidenceIDs[0] || records[0].RunID != in.RunID {
			t.Fatalf("%#v", records)
		}
		if len(model.Reports()) != 1 || model.Reports()[0].ProviderID != "fixture" {
			t.Fatalf("%#v", model.Reports())
		}
	})
	t.Run("uncertain result", func(t *testing.T) {
		resetEval()
		UseDebugFixtures()
		in := evalInput("66666666-6666-4666-8666-666666666666", []string{"architecture-notes"}, strings.Repeat("b", 64))
		got := runEval(t, in)
		if got.Decision != "complete" || got.Outcome != "insufficient_evidence" || len(got.EvidenceIDs) != 0 {
			t.Fatalf("%#v", got)
		}
		if len(mcp.EvidenceRecords()) != 0 {
			t.Fatalf("%#v", mcp.EvidenceRecords())
		}
		reports := model.Reports()
		if len(reports) != 1 || reports[0].Outcome != "insufficient_evidence" || len(reports[0].Limitations) == 0 {
			t.Fatalf("%#v", reports)
		}
	})
	t.Run("failed step", func(t *testing.T) {
		resetEval()
		counter := &denyCounter{}
		SetCorpus(counter)
		SetModel(model.Fixture{})
		in := evalInput("77777777-7777-4777-8777-777777777777", []string{"platform-brief"}, "short")
		got := runEval(t, in)
		if got.Decision != "fail_closed" || got.Reason != "tool_denied" || counter.calls != 0 || len(mcp.EvidenceRecords()) != 0 {
			t.Fatalf("%#v calls=%d", got, counter.calls)
		}
	})
}

func TestFixtureUsageReconcilesOnce(t *testing.T) {
	audit.Reset()
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
	})
	resetEval()
	UseDebugFixtures()
	got := runEval(t, evalInput("45454545-4545-4455-8455-454545454545", []string{"platform-brief"}, strings.Repeat("e", 64)))
	reports := model.Reports()
	prompt, completion, tools := audit.Totals()
	if len(reports) != 1 || prompt != reports[0].Usage.PromptTokens || completion != reports[0].Usage.CompletionTokens || prompt == 0 || tools != 1 || got.Usage.PromptTokens != prompt {
		t.Fatalf("meter %d %d tools %d report %#v result %#v", prompt, completion, tools, reports, got)
	}
	encoded, _ := json.Marshal(audit.Events())
	if strings.Contains(string(encoded), "Which service owns execution?") || strings.Contains(string(encoded), "Bearer") {
		t.Fatal(string(encoded))
	}
}

func TestReplayDoesNotCallToolsAndReexecutionDoes(t *testing.T) {
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
	})
	calls := 0
	SetCorpus(countingTool{&calls})
	SetModel(model.Fixture{})
	ResetActivityEntries()
	first := evalInput("88888888-8888-4888-8888-888888888888", []string{"platform-brief"}, strings.Repeat("c", 64))
	before := ActivityEntries()
	runEval(t, first)
	afterFirst := ActivityEntries()
	if afterFirst <= before || calls == 0 {
		t.Fatalf("re-execution did not run activities entries=%d calls=%d", afterFirst, calls)
	}
	second := evalInput("99999999-9999-4999-8999-999999999999", []string{"platform-brief"}, strings.Repeat("c", 64))
	runEval(t, second)
	if second.WorkflowID == first.WorkflowID || ActivityEntries() <= afterFirst || calls < 2 {
		t.Fatal("second execution did not use a new workflow")
	}
	ResetActivityEntries()
	calls = 0
	in := evalInput("33333333-3333-4333-8333-333333333333", []string{"platform-brief"}, strings.Repeat("a", 64))
	history := successHistory(t, in)
	var decoded Correlation
	if err := converter.GetDefaultDataConverter().FromPayloads(history.Events[0].GetWorkflowExecutionStartedEventAttributes().GetInput(), &decoded); err != nil || decoded.WorkflowID != in.WorkflowID {
		t.Fatal(err, decoded)
	}
	if err := Replay(history, in.WorkflowID); err != nil {
		t.Fatal(err)
	}
	if ActivityEntries() != 0 || calls != 0 {
		t.Fatalf("replay called activities entries=%d calls=%d", ActivityEntries(), calls)
	}
}

type countingTool struct{ calls *int }

func (c countingTool) Call(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	*c.calls++
	return mcp.FixtureTool{}.Call(ctx, name, args)
}

type denyCounter struct{ calls int }

func (d *denyCounter) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	d.calls++
	return nil, nil
}

func resetEval() {
	mcp.Reset()
	model.Reset()
	ResetCommits()
	ResetTerminals()
	ResetActivityEntries()
}

func evalInput(runID string, documents []string, snapshot string) Correlation {
	return Correlation{
		RunID: runID, WorkflowID: "forge-run/" + runID,
		WorkloadID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
		CommandID: "44444444-4444-4444-8444-444444444444", SnapshotID: snapshot,
		Question: "Which service owns execution?", DocumentIDs: documents, MaxPassages: 8,
	}
}

func runEval(t *testing.T, in Correlation) Result {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
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

func successHistory(t *testing.T, in Correlation) *historypb.History {
	t.Helper()
	passage := mcp.Passage{
		DocumentID: "platform-brief", PassageID: "passage-1", EvidenceID: "33333333-3333-4333-8333-333333333333",
		TextSHA256: strings.Repeat("d", 64), Excerpt: "synthetic",
	}
	retrieved := Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded", Passages: []mcp.Passage{passage}}
	usage := model.Usage{PromptTokens: 4, CompletionTokens: 2}
	generated := Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "succeeded", Class: "answered", Usage: usage}
	terminal := Terminal{RunID: in.RunID, WorkflowID: in.WorkflowID, State: "complete", Compensation: "none"}
	result := Result{
		RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Decision: "complete", Reason: "answered",
		Outcome: "answered", RequestedID: model.RequestedModel, EvidenceIDs: []string{passage.EvidenceID}, Usage: usage,
	}
	queue := &taskqueuepb.TaskQueue{Name: TaskQueue}
	when := time.Unix(1_700_000_000, 0).UTC()
	stamp := func(id int64, kind enumspb.EventType) *historypb.HistoryEvent {
		return &historypb.HistoryEvent{EventId: id, EventTime: timestamppb.New(when), EventType: kind}
	}
	wft := func(scheduled, started, completed int64) []*historypb.HistoryEvent {
		first := stamp(scheduled, enumspb.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED)
		first.Attributes = &historypb.HistoryEvent_WorkflowTaskScheduledEventAttributes{WorkflowTaskScheduledEventAttributes: &historypb.WorkflowTaskScheduledEventAttributes{TaskQueue: queue}}
		second := stamp(started, enumspb.EVENT_TYPE_WORKFLOW_TASK_STARTED)
		second.Attributes = &historypb.HistoryEvent_WorkflowTaskStartedEventAttributes{WorkflowTaskStartedEventAttributes: &historypb.WorkflowTaskStartedEventAttributes{ScheduledEventId: scheduled}}
		third := stamp(completed, enumspb.EVENT_TYPE_WORKFLOW_TASK_COMPLETED)
		third.Attributes = &historypb.HistoryEvent_WorkflowTaskCompletedEventAttributes{WorkflowTaskCompletedEventAttributes: &historypb.WorkflowTaskCompletedEventAttributes{ScheduledEventId: scheduled, StartedEventId: started}}
		return []*historypb.HistoryEvent{first, second, third}
	}
	activity := func(scheduled int64, activityID, name string, input, output *commonpb.Payloads) []*historypb.HistoryEvent {
		started := scheduled + 1
		completed := scheduled + 2
		first := stamp(scheduled, enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED)
		first.Attributes = &historypb.HistoryEvent_ActivityTaskScheduledEventAttributes{ActivityTaskScheduledEventAttributes: &historypb.ActivityTaskScheduledEventAttributes{
			ActivityId: activityID, ActivityType: &commonpb.ActivityType{Name: name}, TaskQueue: queue, Input: input,
		}}
		second := stamp(started, enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED)
		second.Attributes = &historypb.HistoryEvent_ActivityTaskStartedEventAttributes{ActivityTaskStartedEventAttributes: &historypb.ActivityTaskStartedEventAttributes{ScheduledEventId: scheduled, Attempt: 1}}
		third := stamp(completed, enumspb.EVENT_TYPE_ACTIVITY_TASK_COMPLETED)
		third.Attributes = &historypb.HistoryEvent_ActivityTaskCompletedEventAttributes{ActivityTaskCompletedEventAttributes: &historypb.ActivityTaskCompletedEventAttributes{ScheduledEventId: scheduled, StartedEventId: started, Result: output}}
		return []*historypb.HistoryEvent{first, second, third}
	}
	started := stamp(1, enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED)
	started.Attributes = &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
		WorkflowType: &commonpb.WorkflowType{Name: WorkflowName}, TaskQueue: queue, Input: mustPayloads(t, in),
	}}
	events := []*historypb.HistoryEvent{started}
	events = append(events, wft(2, 3, 4)...)
	events = append(events, activity(5, "5", "Retrieve", mustPayloads(t, in), mustPayloads(t, retrieved))...)
	events = append(events, wft(8, 9, 10)...)
	events = append(events, activity(11, "11", "Generate", mustPayloads(t, in, retrieved.Passages), mustPayloads(t, generated))...)
	events = append(events, wft(14, 15, 16)...)
	events = append(events, activity(17, "17", "RecordTerminal", mustPayloads(t, terminal), mustPayloads(t))...)
	events = append(events, wft(20, 21, 22)...)
	completed := stamp(23, enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED)
	completed.Attributes = &historypb.HistoryEvent_WorkflowExecutionCompletedEventAttributes{WorkflowExecutionCompletedEventAttributes: &historypb.WorkflowExecutionCompletedEventAttributes{
		WorkflowTaskCompletedEventId: 22, Result: mustPayloads(t, result),
	}}
	events = append(events, completed)
	return &historypb.History{Events: events}
}

func mustPayloads(t *testing.T, values ...any) *commonpb.Payloads {
	t.Helper()
	encoded, err := converter.GetDefaultDataConverter().ToPayloads(values...)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
