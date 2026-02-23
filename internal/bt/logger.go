package bt

// Logger provides structured logging for the service layer.
// The args follow slog conventions: alternating key/value pairs.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NopLogger is a Logger that discards all output. Use in tests.
type NopLogger struct{}

func NewNopLogger() *NopLogger { return &NopLogger{} }

func (*NopLogger) Debug(string, ...any) {}
func (*NopLogger) Info(string, ...any)  {}
func (*NopLogger) Warn(string, ...any)  {}
func (*NopLogger) Error(string, ...any) {}
