package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// btHandler is a custom slog.Handler that formats log records as:
//
//	<timestamp>\t<level>\t<opID>\t<message>\t<key=value ...>
type btHandler struct {
	w     io.Writer
	opID  string
	attrs []slog.Attr
}

func (h *btHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *btHandler) Handle(_ context.Context, r slog.Record) error {
	ts := r.Time.UTC().Format("2006-01-02T15:04:05Z")
	level := r.Level.String()

	_, err := fmt.Fprintf(h.w, "%s\t%s\t%s\t%s", ts, level, h.opID, r.Message)
	if err != nil {
		return err
	}

	// Write pre-set attrs.
	for _, a := range h.attrs {
		fmt.Fprintf(h.w, "\t%s=%v", a.Key, a.Value)
	}

	// Write per-record attrs.
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, "\t%s=%v", a.Key, a.Value)
		return true
	})

	_, err = fmt.Fprintln(h.w)
	return err
}

func (h *btHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &btHandler{
		w:     h.w,
		opID:  h.opID,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *btHandler) WithGroup(string) slog.Handler { return h }

// newLogger creates a structured logger that writes to both logDir/bt.log and stderr.
// It returns the slog.Logger, the open log file (for cleanup), and any error.
func newLogger(logDir string, opID string) (*slog.Logger, *os.File, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "bt.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	w := io.MultiWriter(f, os.Stderr)
	handler := &btHandler{w: w, opID: opID}
	return slog.New(handler), f, nil
}

// slogAdapter wraps *slog.Logger to satisfy the bt.Logger interface.
type slogAdapter struct {
	l *slog.Logger
}

func (a *slogAdapter) Debug(msg string, args ...any) { a.l.Debug(msg, args...) }
func (a *slogAdapter) Info(msg string, args ...any)  { a.l.Info(msg, args...) }
func (a *slogAdapter) Warn(msg string, args ...any)  { a.l.Warn(msg, args...) }
func (a *slogAdapter) Error(msg string, args ...any) { a.l.Error(msg, args...) }
