package devmetrics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/example/devtools-metrics-bridge/internal/infrai"
)

type Reporter interface {
	ReportMetric(context.Context, string, string, float64, map[string]string, string) error
	UsageTimeseries(context.Context, any) error
}

type Event struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Project  string  `json:"project"`
	Outcome  string  `json:"outcome,omitempty"`
	Severity string  `json:"severity,omitempty"`
	Value    float64 `json:"value,omitempty"`
}

type metric struct {
	Type  string            `json:"type"`
	Name  string            `json:"name"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags"`
}

type point struct {
	At     time.Time `json:"at"`
	Metric metric    `json:"metric"`
}

type Service struct {
	reporter Reporter
	now      func() time.Time
	mu       sync.RWMutex
	points   []point
}

func New(reporter Reporter) *Service {
	return &Service{reporter: reporter, now: time.Now}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.handleEvent)
	mux.HandleFunc("GET /chart", s.handleChart)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func mapEvent(event Event) (metric, error) {
	tags := map[string]string{"project": event.Project}
	switch event.Kind {
	case "build":
		if event.Outcome == "" {
			return metric{}, errors.New("build outcome is required")
		}
		tags["outcome"] = event.Outcome
		return metric{Type: "counter", Name: "devtools.builds", Value: 1, Tags: tags}, nil
	case "release":
		if event.Outcome == "" {
			return metric{}, errors.New("release outcome is required")
		}
		tags["outcome"] = event.Outcome
		return metric{Type: "counter", Name: "devtools.releases", Value: 1, Tags: tags}, nil
	case "diagnostic":
		if event.Severity == "" || event.Value < 0 {
			return metric{}, errors.New("diagnostic severity and a non-negative value are required")
		}
		tags["severity"] = event.Severity
		return metric{Type: "gauge", Name: "devtools.open_diagnostics", Value: event.Value, Tags: tags}, nil
	default:
		return metric{}, errors.New("kind must be build, release, or diagnostic")
	}
}

func (s *Service) handleEvent(w http.ResponseWriter, r *http.Request) {
	var event Event
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON event"})
		return
	}
	if event.ID == "" || event.Project == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and project are required"})
		return
	}
	m, err := mapEvent(event)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	key := eventKey(event)
	if err := s.reporter.ReportMetric(r.Context(), m.Type, m.Name, m.Value, m.Tags, key); err != nil {
		status := http.StatusBadGateway
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			status = apiErr.Status
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	s.mu.Lock()
	s.points = append(s.points, point{At: s.now().UTC(), Metric: m})
	s.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "metric": m.Name})
}

func (s *Service) handleChart(w http.ResponseWriter, r *http.Request) {
	var usage any
	if err := s.reporter.UsageTimeseries(r.Context(), &usage); err != nil {
		status := http.StatusBadGateway
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			status = apiErr.Status
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	s.mu.RLock()
	points := append([]point(nil), s.points...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"business_metrics": points, "platform_usage": usage})
}

func eventKey(event Event) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s", event.Project, event.Kind, event.ID)))
	return "dev-event-" + hex.EncodeToString(sum[:16])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
