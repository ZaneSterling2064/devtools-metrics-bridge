package devmetrics

import (
	"context"
	"reflect"
	"testing"
)

type recordingReporter struct {
	got metric
	key string
}

func (r *recordingReporter) ReportMetric(_ context.Context, typ, name string, value float64, tags map[string]string, key string) error {
	r.got = metric{Type: typ, Name: name, Value: value, Tags: tags}
	r.key = key
	return nil
}

func (*recordingReporter) UsageTimeseries(_ context.Context, dst any) error { return nil }

func TestMapEventBusinessDecision(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  metric
	}{
		{"successful build is a counter", Event{Kind: "build", Project: "compiler", Outcome: "passed"}, metric{Type: "counter", Name: "devtools.builds", Value: 1, Tags: map[string]string{"project": "compiler", "outcome": "passed"}}},
		{"release operation is a counter", Event{Kind: "release", Project: "cli", Outcome: "published"}, metric{Type: "counter", Name: "devtools.releases", Value: 1, Tags: map[string]string{"project": "cli", "outcome": "published"}}},
		{"open diagnostics is a gauge", Event{Kind: "diagnostic", Project: "language-server", Severity: "warning", Value: 7}, metric{Type: "gauge", Name: "devtools.open_diagnostics", Value: 7, Tags: map[string]string{"project": "language-server", "severity": "warning"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapEvent(tt.event)
			if err != nil {
				t.Fatalf("mapEvent() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mapEvent() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestEventKeyIsStableAndEventSpecific(t *testing.T) {
	a := Event{ID: "build-42", Kind: "build", Project: "compiler"}
	b := Event{ID: "build-43", Kind: "build", Project: "compiler"}
	if eventKey(a) != eventKey(a) {
		t.Fatal("same event produced different idempotency keys")
	}
	if eventKey(a) == eventKey(b) {
		t.Fatal("different events produced the same idempotency key")
	}
}
