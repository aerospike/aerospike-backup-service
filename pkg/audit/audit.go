package audit

import (
	"context"
	"log/slog"
)

// EventStatus represents the outcome of an audited operation.
type EventStatus string

const (
	StatusSuccess EventStatus = "Success"
	StatusFailed  EventStatus = "Failed"
	StatusDenied  EventStatus = "Denied" // For future RBAC
)

// Auditor provides an interface for emitting structured audit events.
type Auditor interface {
	// WriteEvent records an audit log entry.
	// action: The logical operation (e.g., "AddAerospikeCluster")
	// status: Outcome of the operation
	// attrs: Additional safe, structured metadata
	WriteEvent(ctx context.Context, action string, status EventStatus, attrs ...slog.Attr)
}

// SimpleAuditor is a basic implementation of Auditor that writes to a slog.Logger.
type SimpleAuditor struct {
	logger *slog.Logger
}

// NewSimpleAuditor creates a new SimpleAuditor.
func NewSimpleAuditor(logger *slog.Logger) *SimpleAuditor {
	if logger == nil {
		logger = slog.Default()
	}
	return &SimpleAuditor{
		logger: logger,
	}
}

// WriteEvent records an audit log entry using the underlying logger.
func (a *SimpleAuditor) WriteEvent(ctx context.Context, action string, status EventStatus, attrs ...slog.Attr) {
	allAttrs := make([]any, 0, len(attrs)+2)
	allAttrs = append(allAttrs, slog.String("action", action), slog.String("status", string(status)))
	for _, attr := range attrs {
		allAttrs = append(allAttrs, attr)
	}

	a.logger.InfoContext(ctx, "Audit Event", allAttrs...)
}
