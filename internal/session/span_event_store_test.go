package session
// white-box-reason: OTel instrumentation: tests unexported SpanEventStore wrapper and attribute inspection

import (
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

// stubEventStore implements port.EventStore with canned responses for span tests.
type stubEventStore struct {
	appendResult domain.AppendResult
	loadEvents   []domain.Event
	loadResult   domain.LoadResult
}

func (s *stubEventStore) Append(_ ...domain.Event) (domain.AppendResult, error) {
	return s.appendResult, nil
}

func (s *stubEventStore) LoadAll() ([]domain.Event, domain.LoadResult, error) {
	return s.loadEvents, s.loadResult, nil
}

func (s *stubEventStore) LoadSince(_ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return s.loadEvents, s.loadResult, nil
}

// findSpanByName returns the first span stub matching the given name, or nil.
func findSpanByName(spans tracetest.SpanStubs, name string) *tracetest.SpanStub {
	for i := range spans {
		if spans[i].Name == name {
			return &spans[i]
		}
	}
	return nil
}

// hasAttribute checks whether a span stub contains a given attribute key.
func hasAttribute(span *tracetest.SpanStub, key string) bool {
	for _, attr := range span.Attributes {
		if string(attr.Key) == key {
			return true
		}
	}
	return false
}

func TestSpanEventStore_BasicMode_OmitsDebugAttributes(t *testing.T) {
	exp := setupTestTracer(t)

	// given — basic mode
	old := platform.OTELDetailLevel
	platform.OTELDetailLevel = platform.DetailBasic
	t.Cleanup(func() { platform.OTELDetailLevel = old })

	stub := &stubEventStore{
		appendResult: domain.AppendResult{BytesWritten: 42},
		loadResult:   domain.LoadResult{FileCount: 3, CorruptLineCount: 1},
		loadEvents:   []domain.Event{{Type: "test"}},
	}
	store := NewSpanEventStore(stub).(*SpanEventStore)

	// when
	store.Append(domain.Event{Type: "test"})
	store.LoadAll()

	// then — basic attributes present, debug attributes absent
	spans := exp.GetSpans()

	appendSpan := findSpanByName(spans, "eventsource.append")
	if appendSpan == nil {
		t.Fatal("missing eventsource.append span")
	}
	if !hasAttribute(appendSpan, "event.count.in") {
		t.Error("basic attribute event.count.in missing in basic mode")
	}
	if hasAttribute(appendSpan, "event.append.bytes") {
		t.Error("debug attribute event.append.bytes present in basic mode")
	}

	loadSpan := findSpanByName(spans, "eventsource.load_all")
	if loadSpan == nil {
		t.Fatal("missing eventsource.load_all span")
	}
	if !hasAttribute(loadSpan, "event.count.out") {
		t.Error("basic attribute event.count.out missing in basic mode")
	}
	if hasAttribute(loadSpan, "event.file.count") {
		t.Error("debug attribute event.file.count present in basic mode")
	}
	if hasAttribute(loadSpan, "event.corrupt_line.count") {
		t.Error("debug attribute event.corrupt_line.count present in basic mode")
	}
}

func TestSpanEventStore_DebugMode_IncludesDebugAttributes(t *testing.T) {
	exp := setupTestTracer(t)

	// given — debug mode
	old := platform.OTELDetailLevel
	platform.OTELDetailLevel = platform.DetailDebug
	t.Cleanup(func() { platform.OTELDetailLevel = old })

	stub := &stubEventStore{
		appendResult: domain.AppendResult{BytesWritten: 42},
		loadResult:   domain.LoadResult{FileCount: 3, CorruptLineCount: 1},
		loadEvents:   []domain.Event{{Type: "test"}},
	}
	store := NewSpanEventStore(stub).(*SpanEventStore)

	// when
	store.Append(domain.Event{Type: "test"})
	store.LoadAll()

	// then — all attributes present
	spans := exp.GetSpans()

	appendSpan := findSpanByName(spans, "eventsource.append")
	if appendSpan == nil {
		t.Fatal("missing eventsource.append span")
	}
	if !hasAttribute(appendSpan, "event.count.in") {
		t.Error("basic attribute event.count.in missing")
	}
	if !hasAttribute(appendSpan, "event.append.bytes") {
		t.Error("debug attribute event.append.bytes missing in debug mode")
	}

	loadSpan := findSpanByName(spans, "eventsource.load_all")
	if loadSpan == nil {
		t.Fatal("missing eventsource.load_all span")
	}
	if !hasAttribute(loadSpan, "event.count.out") {
		t.Error("basic attribute event.count.out missing")
	}
	if !hasAttribute(loadSpan, "event.file.count") {
		t.Error("debug attribute event.file.count missing in debug mode")
	}
	if !hasAttribute(loadSpan, "event.corrupt_line.count") {
		t.Error("debug attribute event.corrupt_line.count missing in debug mode")
	}
}

func TestSpanEventStore_NoPII_InAttributes(t *testing.T) {
	exp := setupTestTracer(t)

	// given — debug mode (maximum attributes)
	old := platform.OTELDetailLevel
	platform.OTELDetailLevel = platform.DetailDebug
	t.Cleanup(func() { platform.OTELDetailLevel = old })

	secretData := json.RawMessage(`{"secret":"password123"}`)
	stub := &stubEventStore{
		appendResult: domain.AppendResult{BytesWritten: len(secretData)},
		loadResult:   domain.LoadResult{FileCount: 1, CorruptLineCount: 0},
		loadEvents:   []domain.Event{{Type: "test", Data: secretData}},
	}
	store := NewSpanEventStore(stub).(*SpanEventStore)

	// when
	store.Append(domain.Event{Type: "test", Data: secretData})
	store.LoadAll()

	// then — no attribute value contains event body or PII-like data
	spans := exp.GetSpans()
	for _, s := range spans {
		for _, attr := range s.Attributes {
			val := attr.Value.AsString()
			if val == "" {
				continue
			}
			if val == "password123" || val == string(secretData) {
				t.Errorf("PII/body leak in span %q attr %q = %q", s.Name, attr.Key, val)
			}
		}
	}
}
