package log

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a structured log entry.
type LogEntry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"msg"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// LogCaptureHandler captures log records in a circular buffer.
type LogCaptureHandler struct {
	buffer *ringBuffer
	attrs  []slog.Attr
	groups []string
}

// NewLogCaptureHandler creates a new LogCaptureHandler with the specified capacity.
func NewLogCaptureHandler(capacity int) *LogCaptureHandler {
	return &LogCaptureHandler{
		buffer: newRingBuffer(capacity),
	}
}

// Enabled always returns true for LogCaptureHandler as it captures everything.
// Filtering can be done when querying.
func (h *LogCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle converts the record to a LogEntry and stores it.
func (h *LogCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	// Add handler attributes
	for _, attr := range h.attrs {
		h.addToMap(entry.Attrs, attr, h.groups)
	}

	// Add record attributes
	r.Attrs(func(attr slog.Attr) bool {
		h.addToMap(entry.Attrs, attr, h.groups)
		return true
	})

	h.buffer.add(entry)

	return nil
}

// addToMap adds an attribute to the map, respecting groups.
func (h *LogCaptureHandler) addToMap(m map[string]any, attr slog.Attr, groups []string) {
	if attr.Key == "" {
		return
	}

	val := attr.Value.Resolve()
	// Navigate/Create groups
	currentMap := m
	for _, group := range groups {
		if _, ok := currentMap[group]; !ok {
			currentMap[group] = make(map[string]any)
		}
		if nextMap, ok := currentMap[group].(map[string]any); ok {
			currentMap = nextMap
		} else {
			// Conflict: group name exists but is not a map. Overwrite or ignore?
			// For simplicity, we ignore or basic overwrite.
			// Ideally we shouldn't have conflicts in well-structured logs.
			// Let's reset it to map.
			newMap := make(map[string]any)
			currentMap[group] = newMap
			currentMap = newMap
		}
	}

	currentMap[attr.Key] = val.Any()
}

// WithAttrs returns a new handler with the attributes.
func (h *LogCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogCaptureHandler{
		buffer: h.buffer,
		attrs:  append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
		groups: h.groups,
	}
}

// WithGroup returns a new handler with the group.
func (h *LogCaptureHandler) WithGroup(name string) slog.Handler {
	return &LogCaptureHandler{
		buffer: h.buffer,
		attrs:  h.attrs,
		groups: append(h.groups[:len(h.groups):len(h.groups)], name),
	}
}

// GetEntries returns a copy of the captured log entries.
func (h *LogCaptureHandler) GetEntries() []LogEntry {
	return h.buffer.get()
}

// ringBuffer is a thread-safe circular buffer for LogEntry.
type ringBuffer struct {
	entries []LogEntry
	size    int
	mu      sync.RWMutex
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		entries: make([]LogEntry, 0, size),
		size:    size,
	}
}

func (rb *ringBuffer) add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries = append(rb.entries, entry)
	if len(rb.entries) > rb.size {
		// Drop oldest. Efficient enough for this use case.
		// For very high throughput, we might use a proper ring index,
		// but slice manipulation is fine for 1000 items.
		rb.entries = rb.entries[len(rb.entries)-rb.size:]
	}
}

func (rb *ringBuffer) get() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]LogEntry, len(rb.entries))
	copy(result, rb.entries)
	return result
}
