package infrai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientEnvelopeAndRetryBoundary(t *testing.T) {
	tests := []struct {
		name       string
		responses  []string
		statuses   []int
		wantCalls  int
		wantStatus int
	}{
		{"business rejection keeps its status", []string{`{"ok":false,"data":null,"error":{"message":"request rejected"},"metadata":{}}`}, []int{422}, 1, 422},
		{"rate limit retries then succeeds", []string{`{"ok":false,"data":null,"error":{"message":"retry later"},"metadata":{}}`, `{"ok":true,"data":{},"error":null,"metadata":{}}`}, []int{429, 200}, 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/metrics/report" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "event-1" {
					t.Error("expected bearer auth and idempotency headers")
				}
				i := calls
				calls++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(tt.statuses[i])
				_, _ = w.Write([]byte(tt.responses[i]))
			}))
			defer server.Close()

			client := New(server.URL, "test-key", server.Client())
			client.sleep = func(context.Context, time.Duration) error { return nil }
			err := client.ReportMetric(context.Background(), "counter", "devtools.builds", 1, map[string]string{"project": "compiler"}, "event-1")
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
			var apiErr *APIError
			if tt.wantStatus == 0 && err != nil {
				t.Fatalf("ReportMetric() error = %v", err)
			}
			if tt.wantStatus != 0 && (!errors.As(err, &apiErr) || apiErr.Status != tt.wantStatus) {
				t.Fatalf("ReportMetric() error = %#v, want API status %d", err, tt.wantStatus)
			}
		})
	}
}
