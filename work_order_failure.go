package fieldservice

import (
	"context"
	"fmt"
)

type WorkOrderFailure struct {
	WorkOrderID    string `json:"work_order_id"`
	PhotoID        string `json:"photo_id"`
	DispatchStatus string `json:"dispatch_status"`
	TechnicianID   string `json:"technician_id"`
	Operation      string `json:"operation"`
	Exception      string `json:"exception"`
}

type ErrorCapturer interface {
	Capture(context.Context, string, CaptureRequest) (CaptureResult, error)
}

type FollowUp struct {
	Required     bool   `json:"required"`
	Reason       string `json:"reason"`
	EventID      string `json:"event_id"`
	ErrorGroupID string `json:"error_group_id"`
}

func CaptureWorkOrderFailure(ctx context.Context, capturer ErrorCapturer, failure WorkOrderFailure) (FollowUp, error) {
	fingerprint := []string{"field-service", failure.Operation, failure.DispatchStatus}
	result, err := capturer.Capture(ctx, "work-order:"+failure.WorkOrderID+":"+failure.Operation, CaptureRequest{
		Title:       "Work-order operation failed",
		Message:     failure.Operation + " failed for work order " + failure.WorkOrderID,
		Level:       "error",
		Fingerprint: fingerprint,
		Exception:   failure.Exception,
		Context: map[string]any{
			"work_order_id":   failure.WorkOrderID,
			"photo_id":        failure.PhotoID,
			"dispatch_status": failure.DispatchStatus,
			"technician_id":   failure.TechnicianID,
			"operation":       failure.Operation,
		},
	})
	if err != nil {
		return FollowUp{}, fmt.Errorf("capture work-order failure: %w", err)
	}

	return FollowUp{
		Required:     failure.DispatchStatus == "technician_assigned",
		Reason:       "captured and grouped by operation plus dispatch status",
		EventID:      result.EventID,
		ErrorGroupID: result.ErrorGroupID,
	}, nil
}
