package log

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Entry represents a structured log entry.
// @Description Entry represents a structured log entry.
type Entry struct {
	// Time is the time of the log entry.
	Time time.Time `json:"time" example:"2006-01-02T15:04:05Z07:00"`
	// Level is the log level.
	Level string `json:"level" example:"INFO"`
	// Message is the log message.
	Message string `json:"msg" example:"Hello world!"`
	// Attrs is a map of additional attributes.
	Attrs map[string]any `json:"attrs,omitempty"`
}

// CaptureHandler captures log records in a circular buffer.
type CaptureHandler struct {
	buffer *ringBuffer
	attrs  []slog.Attr
	groups []string
}

// NewCaptureHandler creates a new CaptureHandler with the specified capacity.
func NewCaptureHandler(capacity int) *CaptureHandler {
	return &CaptureHandler{
		buffer: newRingBuffer(capacity),
	}
}

// Enabled always returns true for CaptureHandler as it captures everything.
// Filtering can be done when querying.
func (h *CaptureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle converts the record to a Entry and stores it.
func (h *CaptureHandler) Handle(_ context.Context, r slog.Record) error {
	entry := Entry{
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
func (h *CaptureHandler) addToMap(m map[string]any, attr slog.Attr, groups []string) {
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
func (h *CaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CaptureHandler{
		buffer: h.buffer,
		attrs:  append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
		groups: h.groups,
	}
}

// WithGroup returns a new handler with the group.
func (h *CaptureHandler) WithGroup(name string) slog.Handler {
	return &CaptureHandler{
		buffer: h.buffer,
		attrs:  h.attrs,
		groups: append(h.groups[:len(h.groups):len(h.groups)], name),
	}
}

// GetEntries returns a copy of the captured log entries.
func (h *CaptureHandler) GetEntries() []Entry {
	return h.buffer.get()
}

// ringBuffer is a thread-safe circular buffer for Entry.
type ringBuffer struct {
	entries []Entry
	size    int
	mu      sync.RWMutex
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		entries: make([]Entry, 0, size),
		size:    size,
	}
}

func (rb *ringBuffer) add(entry Entry) {
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

func (rb *ringBuffer) get() []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]Entry, len(rb.entries))
	copy(result, rb.entries)
	return result
}
