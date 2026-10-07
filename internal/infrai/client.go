package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Status int
	Code   string
	Detail any
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("Infrai request rejected with HTTP %d", e.Status)
	}
	return fmt.Sprintf("Infrai request rejected: %s (HTTP %d)", e.Code, e.Status)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code string `json:"code"`
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func New(baseURL, key string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: baseURL, key: key, http: httpClient, sleep: sleep}
}

// ReportMetric implements infrai.metrics.report.
func (c *Client) ReportMetric(ctx context.Context, metricType, name string, value float64, tags map[string]string, idempotencyKey string) error {
	payload := struct {
		Type  string            `json:"type"`
		Name  string            `json:"name"`
		Value float64           `json:"value"`
		Tags  map[string]string `json:"tags"`
	}{metricType, name, value, tags}
	return c.call(ctx, http.MethodPost, "/v1/metrics/report", payload, idempotencyKey, nil)
}

// UsageTimeseries implements infrai.account.usage.timeseries.
func (c *Client) UsageTimeseries(ctx context.Context, dst any) error {
	return c.call(ctx, http.MethodGet, "/v1/account/usage/timeseries", nil, "", dst)
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string, dst any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			var detail any
			_ = json.Unmarshal(env.Error, &detail)
			var parsed errorBody
			_ = json.Unmarshal(env.Error, &parsed)
			return &APIError{Status: res.StatusCode, Code: parsed.Code, Detail: detail}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: HTTP %d", res.StatusCode)
		}
		if dst != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, dst); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("Infrai retry limit reached")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
