package fieldservice

import (
	"context"
	"reflect"
	"testing"
)

type recordingCapturer struct {
	key   string
	input CaptureRequest
}

func (r *recordingCapturer) Capture(_ context.Context, key string, input CaptureRequest) (CaptureResult, error) {
	r.key, r.input = key, input
	return CaptureResult{EventID: "evt_42", ErrorGroupID: "grp_7"}, nil
}

func TestCaptureWorkOrderFailureGroupingAndFollowUp(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantFollow bool
	}{
		{name: "assigned technician receives follow-up", status: "technician_assigned", wantFollow: true},
		{name: "queued dispatch stays in dispatch flow", status: "queued", wantFollow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingCapturer{}
			got, err := CaptureWorkOrderFailure(context.Background(), recorder, WorkOrderFailure{
				WorkOrderID: "wo-1842", PhotoID: "photo-3", DispatchStatus: tt.status,
				TechnicianID: "tech-9", Operation: "photo_upload", Exception: "image checksum mismatch",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Required != tt.wantFollow {
				t.Fatalf("follow-up = %v, want %v", got.Required, tt.wantFollow)
			}
			wantFingerprint := []string{"field-service", "photo_upload", tt.status}
			if !reflect.DeepEqual(recorder.input.Fingerprint, wantFingerprint) {
				t.Fatalf("fingerprint = %v, want %v", recorder.input.Fingerprint, wantFingerprint)
			}
			if recorder.key != "work-order:wo-1842:photo_upload" {
				t.Fatalf("idempotency key = %q", recorder.key)
			}
		})
	}
}
