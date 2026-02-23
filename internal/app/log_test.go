package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestBtHandler_Handle(t *testing.T) {
	ts := time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name    string
		opID    string
		level   slog.Level
		message string
		attrs   []slog.Attr
		want    string
	}{
		{
			name:    "basic info message",
			opID:    "op-123",
			level:   slog.LevelInfo,
			message: "file backed up",
			want:    "2024-06-15T14:30:45Z\tINFO\top-123\tfile backed up\n",
		},
		{
			name:    "debug level",
			opID:    "op-456",
			level:   slog.LevelDebug,
			message: "checking cache",
			want:    "2024-06-15T14:30:45Z\tDEBUG\top-456\tchecking cache\n",
		},
		{
			name:    "with record attrs",
			opID:    "op-789",
			level:   slog.LevelInfo,
			message: "staged",
			attrs:   []slog.Attr{slog.String("path", "/docs/file.txt"), slog.Int("size", 42)},
			want:    "2024-06-15T14:30:45Z\tINFO\top-789\tstaged\tpath=/docs/file.txt\tsize=42\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := &btHandler{w: &buf, opID: tt.opID}

			r := slog.NewRecord(ts, tt.level, tt.message, 0)
			for _, a := range tt.attrs {
				r.AddAttrs(a)
			}

			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if got := buf.String(); got != tt.want {
				t.Errorf("Handle() output =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestBtHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := &btHandler{w: &buf, opID: "op-1"}

	// Add pre-set attrs
	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "vault")}).(*btHandler)

	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	r := slog.NewRecord(ts, slog.LevelInfo, "upload", 0)
	r.AddAttrs(slog.String("key", "abc"))

	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "component=vault") {
		t.Errorf("expected pre-set attr component=vault, got: %q", got)
	}
	if !strings.Contains(got, "key=abc") {
		t.Errorf("expected record attr key=abc, got: %q", got)
	}
}

func TestBtHandler_WithAttrs_doesNotMutateOriginal(t *testing.T) {
	var buf bytes.Buffer
	h := &btHandler{w: &buf, opID: "op-1", attrs: []slog.Attr{slog.String("a", "1")}}

	h2 := h.WithAttrs([]slog.Attr{slog.String("b", "2")}).(*btHandler)

	if len(h.attrs) != 1 {
		t.Errorf("original handler attrs modified: got %d, want 1", len(h.attrs))
	}
	if len(h2.attrs) != 2 {
		t.Errorf("new handler attrs: got %d, want 2", len(h2.attrs))
	}
}

func TestBtHandler_Enabled(t *testing.T) {
	h := &btHandler{}
	// All levels should be enabled
	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		if !h.Enabled(context.Background(), level) {
			t.Errorf("Enabled(%v) = false, want true", level)
		}
	}
}

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()

	logger, f, err := newLogger(dir, "test-op")
	if err != nil {
		t.Fatalf("newLogger() error = %v", err)
	}
	defer f.Close()

	if logger == nil {
		t.Fatal("newLogger() returned nil logger")
	}
	if f == nil {
		t.Fatal("newLogger() returned nil file")
	}
}
