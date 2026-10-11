// Package telemetry records provisioning and reconciliation measurements.
// It keeps a process-local OpenTelemetry view. It does not run a collector,
// scrape endpoint, or dashboard, and it does not store tokens or corpus text.
package telemetry

import (
	"context"
	"net/http"
	"regexp"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const spanLimit = 32

var (
	safeID      = regexp.MustCompile(`^[0-9a-f]{32}$|^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	knownReason = map[string]struct{}{
		"Reconciling": {}, "ResourceUnavailable": {}, "OwnershipConflict": {},
		"IdentityUnavailable": {}, "OwnerMismatch": {}, "InvalidIdentity": {},
		"NamespacePending": {}, "DefaultAccountPending": {}, "CleanupPending": {},
		"PersistentStoragePresent": {}, "reconcile_error": {},
	}
)

type requestKey struct{}

// Duration is one aggregated sample group.
type Duration struct {
	Count      int64   `json:"count"`
	SumSeconds float64 `json:"sum_seconds"`
}

// Span is a correlation record. It carries request and operation identity only.
type Span struct {
	Name        string `json:"name"`
	TraceID     string `json:"trace_id"`
	RequestID   string `json:"request_id,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
}

// View is the minimal in-process measurement document.
type View struct {
	Provisioning   Area   `json:"provisioning"`
	Reconciliation Area   `json:"reconciliation"`
	Spans          []Span `json:"spans"`
}

// Area separates outcome counts from durations. Reconciliation also keeps
// bounded retry and failure reasons.
type Area struct {
	Outcomes       map[string]int64    `json:"outcomes"`
	Duration       map[string]Duration `json:"duration"`
	RetryReasons   map[string]int64    `json:"retry_reasons,omitempty"`
	FailureReasons map[string]int64    `json:"failure_reasons,omitempty"`
}

type instruments struct {
	reader *sdkmetric.ManualReader
	tracer trace.Tracer
	provN  metric.Int64Counter
	provD  metric.Float64Histogram
	recN   metric.Int64Counter
	recD   metric.Float64Histogram
	recR   metric.Int64Counter
	spans  *ring
}

type ring struct {
	mu    sync.Mutex
	items []Span
}

func (r *ring) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sp := range spans {
		item := Span{Name: sp.Name(), TraceID: sp.SpanContext().TraceID().String()}
		for _, attr := range sp.Attributes() {
			value := attr.Value.AsString()
			if !safeID.MatchString(value) {
				continue
			}
			switch string(attr.Key) {
			case "request_id":
				item.RequestID = value
			case "operation_id":
				item.OperationID = value
			}
		}
		r.items = append(r.items, item)
	}
	if len(r.items) > spanLimit {
		r.items = append([]Span(nil), r.items[len(r.items)-spanLimit:]...)
	}
	return nil
}

func (r *ring) Shutdown(context.Context) error { return nil }

var (
	mu      sync.Mutex
	current *instruments
)

func init() { current = newInstruments() }

// Reset starts a fresh in-process view. Tests use it to isolate samples.
func Reset() {
	mu.Lock()
	current = newInstruments()
	mu.Unlock()
}

func newInstruments() *instruments {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(resource.Empty()),
	)
	meter := provider.Meter("github.com/octieght18/forge")
	boundaries := metric.WithExplicitBucketBoundaries(0.001, 0.01, 0.1, 1, 10, 30, 120)
	provN, _ := meter.Int64Counter("forge.provisioning.outcomes")
	provD, _ := meter.Float64Histogram("forge.provisioning.duration", metric.WithUnit("s"), boundaries)
	recN, _ := meter.Int64Counter("forge.reconciliation.outcomes")
	recD, _ := meter.Float64Histogram("forge.reconciliation.duration", metric.WithUnit("s"), boundaries)
	recR, _ := meter.Int64Counter("forge.reconciliation.reasons")
	spans := &ring{}
	tracer := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Empty()),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(spans)),
	).Tracer("github.com/octieght18/forge")
	return &instruments{reader: reader, tracer: tracer, provN: provN, provD: provD, recN: recN, recD: recD, recR: recR, spans: spans}
}

func active() *instruments {
	mu.Lock()
	defer mu.Unlock()
	return current
}

var traceContext = propagation.TraceContext{}

// Extract continues a W3C traceparent from an incoming request.
func Extract(ctx context.Context, header http.Header) context.Context {
	return traceContext.Extract(ctx, propagation.HeaderCarrier(header))
}

// Inject writes the current span's traceparent onto a response.
func Inject(ctx context.Context, header http.Header) {
	traceContext.Inject(ctx, propagation.HeaderCarrier(header))
}

// WithRequestID returns a context carrying a validated request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if !safeID.MatchString(id) {
		return ctx
	}
	return context.WithValue(ctx, requestKey{}, id)
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestKey{}).(string)
	return id
}

// Start begins a child span. The returned function ends it.
func Start(ctx context.Context, name string) (context.Context, func()) {
	ctx, span := active().tracer.Start(ctx, name)
	return ctx, func() { span.End() }
}

// BeginRequest starts the HTTP span and attaches the request id.
func BeginRequest(ctx context.Context, requestID string) (context.Context, func()) {
	ctx = WithRequestID(ctx, requestID)
	ctx, end := Start(ctx, "forge.http")
	setAttribute(ctx, "request_id", requestID)
	return ctx, end
}

// SetOperation attaches an operation id to the current span.
func SetOperation(ctx context.Context, operationID string) {
	setAttribute(ctx, "operation_id", operationID)
}

func setAttribute(ctx context.Context, key, value string) {
	if !safeID.MatchString(value) {
		return
	}
	trace.SpanFromContext(ctx).SetAttributes(attribute.String(key, value))
}

// RecordProvisioning counts one terminal provisioning result and its duration.
func RecordProvisioning(ctx context.Context, result string, elapsed time.Duration, operationID string) {
	switch result {
	case "ready", "failed", "canceled", "timed_out":
	default:
		result = "unspecified"
	}
	ctx, end := Start(ctx, "forge.provisioning")
	defer end()
	setAttribute(ctx, "request_id", requestIDFrom(ctx))
	setAttribute(ctx, "operation_id", operationID)
	inst := active()
	attrs := metric.WithAttributes(attribute.String("result", result))
	inst.provN.Add(ctx, 1, attrs)
	if elapsed > 0 && elapsed < 15*time.Minute {
		inst.provD.Record(ctx, elapsed.Seconds(), attrs)
	}
}

// RecordReconciliation counts one controller pass. Retry and failure reasons
// use a fixed vocabulary; unknown text becomes unspecified.
func RecordReconciliation(ctx context.Context, elapsed time.Duration, phase, reason string, reconcileErr error) {
	result := "idle"
	switch {
	case reconcileErr != nil:
		result = "failure"
		if reason == "" {
			reason = "reconcile_error"
		}
	case phase == "Ready" || phase == "Absent":
		result = "success"
		reason = ""
	case phase == "Failed":
		result = "failure"
	case phase == "Provisioning" || phase == "Deleting":
		result = "retry"
	default:
		reason = ""
	}
	if reason != "" {
		if _, ok := knownReason[reason]; !ok {
			reason = "unspecified"
		}
	}
	ctx, end := Start(ctx, "forge.reconciliation")
	defer end()
	setAttribute(ctx, "request_id", requestIDFrom(ctx))
	inst := active()
	attrs := metric.WithAttributes(attribute.String("result", result))
	inst.recN.Add(ctx, 1, attrs)
	if elapsed > 0 && elapsed < time.Minute {
		inst.recD.Record(ctx, elapsed.Seconds(), attrs)
	}
	if reason != "" && (result == "retry" || result == "failure") {
		inst.recR.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result), attribute.String("reason", reason)))
	}
}

// Snapshot collects the current process view.
func Snapshot() View {
	inst := active()
	var collected metricdata.ResourceMetrics
	_ = inst.reader.Collect(context.Background(), &collected)
	out := View{
		Provisioning:   emptyArea(false),
		Reconciliation: emptyArea(true),
		Spans:          []Span{},
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, point := range data.DataPoints {
					result := attributeValue(point.Attributes, "result")
					reason := attributeValue(point.Attributes, "reason")
					switch m.Name {
					case "forge.provisioning.outcomes":
						out.Provisioning.Outcomes[result] += point.Value
					case "forge.reconciliation.outcomes":
						out.Reconciliation.Outcomes[result] += point.Value
					case "forge.reconciliation.reasons":
						if result == "retry" {
							out.Reconciliation.RetryReasons[reason] += point.Value
						}
						if result == "failure" {
							out.Reconciliation.FailureReasons[reason] += point.Value
						}
					}
				}
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					result := attributeValue(point.Attributes, "result")
					sample := Duration{Count: int64(point.Count), SumSeconds: point.Sum}
					switch m.Name {
					case "forge.provisioning.duration":
						addDuration(out.Provisioning.Duration, result, sample)
					case "forge.reconciliation.duration":
						addDuration(out.Reconciliation.Duration, result, sample)
					}
				}
			}
		}
	}
	inst.spans.mu.Lock()
	out.Spans = append(out.Spans, inst.spans.items...)
	inst.spans.mu.Unlock()
	return out
}

func emptyArea(reasons bool) Area {
	area := Area{Outcomes: map[string]int64{}, Duration: map[string]Duration{}}
	if reasons {
		area.RetryReasons = map[string]int64{}
		area.FailureReasons = map[string]int64{}
	}
	return area
}

func addDuration(dst map[string]Duration, result string, sample Duration) {
	current := dst[result]
	current.Count += sample.Count
	current.SumSeconds += sample.SumSeconds
	dst[result] = current
}

func attributeValue(set attribute.Set, key string) string {
	value, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}
	return value.AsString()
}
