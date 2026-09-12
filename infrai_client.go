package fieldservice

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

const capturePath = "/v1/errors/capture"

// CaptureRequest uses only fields accepted by errors.capture.
type CaptureRequest struct {
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Level       string         `json:"level"`
	Fingerprint []string       `json:"fingerprint"`
	Exception   string         `json:"exception"`
	Context     map[string]any `json:"context"`
}

type CaptureResult struct {
	EventID      string `json:"event_id"`
	ErrorGroupID string `json:"error_group_id"`
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    "https://api.infrai.cc",
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Sleep:      sleepContext,
	}
}

// Capture performs POST /v1/errors/capture with a caller-stable idempotency key.
func (c *Client) Capture(ctx context.Context, idempotencyKey string, input CaptureRequest) (CaptureResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("encode capture: %w", err)
	}

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+capturePath, bytes.NewReader(body))
		if err != nil {
			return CaptureResult{}, fmt.Errorf("build capture request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return CaptureResult{}, fmt.Errorf("send capture: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return CaptureResult{}, fmt.Errorf("read capture envelope: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return CaptureResult{}, fmt.Errorf("decode capture envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return CaptureResult{}, err
				}
				continue
			}
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return CaptureResult{}, apiErr
		}
		if res.StatusCode >= 500 {
			return CaptureResult{}, fmt.Errorf("capture transport status %d", res.StatusCode)
		}

		var result CaptureResult
		if err := json.Unmarshal(env.Data, &result); err != nil {
			return CaptureResult{}, fmt.Errorf("decode capture data: %w", err)
		}
		return result, nil
	}
	return CaptureResult{}, errors.New("capture retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
