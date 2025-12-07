package log

import (
	"log/slog"
	"sync"
	"testing"
)

func TestLogCaptureHandler(t *testing.T) {
	handler := NewCaptureHandler(5)
	logger := slog.New(handler)

	// Log some entries
	logger.Info("info message")
	logger.Warn("warn message", slog.String("key", "value"))
	logger.Error("error message", slog.Int("code", 123))

	entries := handler.GetEntries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Message != "info message" {
		t.Errorf("expected 'info message', got '%s'", entries[0].Message)
	}
	if entries[0].Level != "INFO" {
		t.Errorf("expected 'INFO', got '%s'", entries[0].Level)
	}

	if entries[1].Message != "warn message" {
		t.Errorf("expected 'warn message', got '%s'", entries[1].Message)
	}
	if entries[1].Attrs["key"] != "value" {
		t.Errorf("expected 'value', got '%v'", entries[1].Attrs["key"])
	}

	// Overflow
	logger.Info("msg 4")
	logger.Info("msg 5")
	logger.Info("msg 6")

	entries = handler.GetEntries()
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
	if entries[0].Message != "warn message" {
		t.Errorf("expected 'warn message', got '%s'", entries[0].Message)
	}
	if entries[4].Message != "msg 6" {
		t.Errorf("expected 'msg 6', got '%s'", entries[4].Message)
	}
}

func TestLogCaptureHandler_WithAttrs(t *testing.T) {
	handler := NewCaptureHandler(10)
	logger := slog.New(handler).With(slog.String("common", "attr"))

	logger.Info("test")

	entries := handler.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Attrs["common"] != "attr" {
		t.Errorf("expected common=attr, got %v", entries[0].Attrs["common"])
	}
}

func TestLogCaptureHandler_WithGroup(t *testing.T) {
	handler := NewCaptureHandler(10)
	logger := slog.New(handler).WithGroup("g")

	logger.Info("test", slog.String("key", "val"))

	entries := handler.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	g, ok := entries[0].Attrs["g"].(map[string]any)
	if !ok {
		t.Fatalf("expected group 'g' to be map[string]any, got %T", entries[0].Attrs["g"])
	}
	if g["key"] != "val" {
		t.Errorf("expected g.key=val, got %v", g["key"])
	}
}

func TestLogCaptureHandler_Concurrency(t *testing.T) {
	handler := NewCaptureHandler(100)
	logger := slog.New(handler)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				logger.Info("log")
			}
		}()
	}
	wg.Wait()

	entries := handler.GetEntries()
	if len(entries) != 100 {
		t.Errorf("expected 100 entries, got %d", len(entries))
	}
}

func TestMultiHandler(t *testing.T) {
	h1 := NewCaptureHandler(10)
	h2 := NewCaptureHandler(10)
	multi := NewMultiHandler(h1, h2)
	logger := slog.New(multi)

	logger.Info("test multi")

	if len(h1.GetEntries()) != 1 {
		t.Error("h1 should have 1 entry")
	}
	if len(h2.GetEntries()) != 1 {
		t.Error("h2 should have 1 entry")
	}
}
