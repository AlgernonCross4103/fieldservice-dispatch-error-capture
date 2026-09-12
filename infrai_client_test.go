package fieldservice

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCaptureDecodesBusinessEnvelopeBeforeStatus(t *testing.T) {
	client := NewClient("test-key")
	client.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != capturePath {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"rejected","message":"work order is required"},"metadata":{}}`)),
		}, nil
	})
	client.Sleep = func(context.Context, time.Duration) error { return nil }

	_, err := client.Capture(context.Background(), "stable-key", CaptureRequest{})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.HTTPStatus != http.StatusBadRequest || apiErr.Code != "rejected" {
		t.Fatalf("error = %#v", err)
	}
}
