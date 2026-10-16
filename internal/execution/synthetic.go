package execution

import (
	"strings"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

// SyntheticSuccessHistory is a completed research history for replay tests.
// It is not a download from a Temporal server.
func SyntheticSuccessHistory(in Correlation) (*historypb.History, error) {
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
	payload := func(values ...any) (*commonpb.Payloads, error) {
		return converter.GetDefaultDataConverter().ToPayloads(values...)
	}
	input, err := payload(in)
	if err != nil {
		return nil, err
	}
	retrievedPayload, err := payload(retrieved)
	if err != nil {
		return nil, err
	}
	generatedInput, err := payload(in, retrieved.Passages)
	if err != nil {
		return nil, err
	}
	generatedPayload, err := payload(generated)
	if err != nil {
		return nil, err
	}
	terminalPayload, err := payload(terminal)
	if err != nil {
		return nil, err
	}
	empty, err := payload()
	if err != nil {
		return nil, err
	}
	resultPayload, err := payload(result)
	if err != nil {
		return nil, err
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
	activity := func(scheduled int64, activityID, name string, in, out *commonpb.Payloads) []*historypb.HistoryEvent {
		started := scheduled + 1
		completed := scheduled + 2
		first := stamp(scheduled, enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED)
		first.Attributes = &historypb.HistoryEvent_ActivityTaskScheduledEventAttributes{ActivityTaskScheduledEventAttributes: &historypb.ActivityTaskScheduledEventAttributes{
			ActivityId: activityID, ActivityType: &commonpb.ActivityType{Name: name}, TaskQueue: queue, Input: in,
		}}
		second := stamp(started, enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED)
		second.Attributes = &historypb.HistoryEvent_ActivityTaskStartedEventAttributes{ActivityTaskStartedEventAttributes: &historypb.ActivityTaskStartedEventAttributes{ScheduledEventId: scheduled, Attempt: 1}}
		third := stamp(completed, enumspb.EVENT_TYPE_ACTIVITY_TASK_COMPLETED)
		third.Attributes = &historypb.HistoryEvent_ActivityTaskCompletedEventAttributes{ActivityTaskCompletedEventAttributes: &historypb.ActivityTaskCompletedEventAttributes{ScheduledEventId: scheduled, StartedEventId: started, Result: out}}
		return []*historypb.HistoryEvent{first, second, third}
	}
	started := stamp(1, enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED)
	started.Attributes = &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
		WorkflowType: &commonpb.WorkflowType{Name: WorkflowName}, TaskQueue: queue, Input: input,
	}}
	events := []*historypb.HistoryEvent{started}
	events = append(events, wft(2, 3, 4)...)
	events = append(events, activity(5, "5", "Retrieve", input, retrievedPayload)...)
	events = append(events, wft(8, 9, 10)...)
	events = append(events, activity(11, "11", "Generate", generatedInput, generatedPayload)...)
	events = append(events, wft(14, 15, 16)...)
	events = append(events, activity(17, "17", "RecordTerminal", terminalPayload, empty)...)
	events = append(events, wft(20, 21, 22)...)
	completed := stamp(23, enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED)
	completed.Attributes = &historypb.HistoryEvent_WorkflowExecutionCompletedEventAttributes{WorkflowExecutionCompletedEventAttributes: &historypb.WorkflowExecutionCompletedEventAttributes{
		WorkflowTaskCompletedEventId: 22, Result: resultPayload,
	}}
	events = append(events, completed)
	return &historypb.History{Events: events}, nil
}
