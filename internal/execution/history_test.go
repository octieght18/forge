package execution

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSanitizeHistoryOmitsPromptsCorpusAndTokens(t *testing.T) {
	when := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	secret := "Bearer super-secret-token Which service owns execution? PostgreSQL owns product records."
	events := []*historypb.HistoryEvent{
		workflowEvent(1, when, enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED),
		{
			EventId: 2, EventTime: timestamppb.New(when.Add(time.Second)), EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_ActivityTaskScheduledEventAttributes{
				ActivityTaskScheduledEventAttributes: &historypb.ActivityTaskScheduledEventAttributes{
					ActivityType: &commonpb.ActivityType{Name: "Retrieve"},
					Input:        &commonpb.Payloads{Payloads: []*commonpb.Payload{{Data: []byte(secret)}}},
				},
			},
		},
		{
			EventId: 3, EventTime: timestamppb.New(when.Add(2 * time.Second)), EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED,
			Attributes: &historypb.HistoryEvent_ActivityTaskStartedEventAttributes{
				ActivityTaskStartedEventAttributes: &historypb.ActivityTaskStartedEventAttributes{ScheduledEventId: 2, Attempt: 2},
			},
		},
		{
			EventId: 4, EventTime: timestamppb.New(when.Add(3 * time.Second)), EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_FAILED,
			Attributes: &historypb.HistoryEvent_ActivityTaskFailedEventAttributes{
				ActivityTaskFailedEventAttributes: &historypb.ActivityTaskFailedEventAttributes{
					ScheduledEventId: 2, RetryState: enumspb.RETRY_STATE_IN_PROGRESS,
					Failure: &failurepb.Failure{
						Message: secret,
						FailureInfo: &failurepb.Failure_ApplicationFailureInfo{ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
							Type: "tool_unavailable",
						}},
					},
				},
			},
		},
		{
			EventId: 5, EventTime: timestamppb.New(when.Add(4 * time.Second)), EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_ActivityTaskScheduledEventAttributes{
				ActivityTaskScheduledEventAttributes: &historypb.ActivityTaskScheduledEventAttributes{
					ActivityType: &commonpb.ActivityType{Name: "Compensate"},
					Input:        &commonpb.Payloads{Payloads: []*commonpb.Payload{{Data: []byte(secret)}}},
				},
			},
		},
	}
	got := SanitizeHistory(events)
	if len(got) != 3 {
		t.Fatalf("events %#v", got)
	}
	if got[0].Step != "workflow" || got[0].EventType != "started" || got[0].Attempt != 1 || got[0].Sequence != 1 {
		t.Fatalf("workflow %#v", got[0])
	}
	if got[1].Step != "retrieval" || got[1].EventType != "started" || got[1].Attempt != 2 || got[1].Failure != nil {
		t.Fatalf("start %#v", got[1])
	}
	if got[2].Step != "retrieval" || got[2].EventType != "retry_scheduled" || got[2].Attempt != 2 || got[2].Failure == nil || got[2].Failure.Code != "tool_unavailable" {
		t.Fatalf("retry %#v", got[2])
	}
	if got[2].Failure.Message != safeMessages["tool_unavailable"] {
		t.Fatalf("message %q", got[2].Failure.Message)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"Bearer", "super-secret-token", "Which service owns execution?", "PostgreSQL owns product records."} {
		if strings.Contains(string(encoded), hidden) {
			t.Fatalf("history leaked %q", hidden)
		}
	}
}

func TestPublicResultDropsUnsafeReasonAndNonIdentifiers(t *testing.T) {
	workflowID := "forge-run/33333333-3333-4333-8333-333333333333"
	view, err := PublicResult(workflowID, Result{
		RunID: "33333333-3333-4333-8333-333333333333", WorkflowID: workflowID, Decision: "complete",
		Reason: "Bearer secret prompt", Outcome: "answered",
		EvidenceIDs: []string{"33333333-3333-4333-8333-333333333333", "../secrets"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Reason != "complete" || view.Outcome != "answered" || len(view.EvidenceIDs) != 1 {
		t.Fatalf("%#v", view)
	}
	if _, err = PublicResult(workflowID, Result{RunID: "other", WorkflowID: workflowID, Decision: "complete"}); err == nil {
		t.Fatal("identity mismatch returned a result")
	}
}

func workflowEvent(id int64, when time.Time, kind enumspb.EventType) *historypb.HistoryEvent {
	return &historypb.HistoryEvent{EventId: id, EventTime: timestamppb.New(when), EventType: kind}
}
