package internal

import (
	"context"
	"log/slog"
	"os"
	"runtime"
)

var Log Logger = noLog{}

func UseSlog() {
	Log = slogLog{logger: slog.New(skipPc{Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr{
			if a.Key == "time" || a.Key == "level" {
				return slog.Attr{}
			}

			return a
		},
	})})}
}

type Logger interface {
	Debug(msg string, keyvals ...interface{})
	With(args ...any) Logger
}

type noLog struct{}

func (n noLog) Debug(_ string, _ ...interface{}) {}

func (n noLog) With(_ ...any) Logger { return n }

type slogLog struct {
	logger *slog.Logger
}

func NewSlogLog(l *slog.Logger) Logger {
	return slogLog{logger: l}
}

func (s slogLog) Debug(msg string, keyvals ...interface{}) {
	s.logger.Debug(msg, keyvals...)
}

func (s slogLog) With(args ...any) Logger {
	return slogLog{logger: s.logger.With(args...)}
}

type skipPc struct {
	slog.Handler
}

func (s skipPc) Enabled(ctx context.Context, level slog.Level) bool {
	return s.Handler.Enabled(ctx, level)
}

func (s skipPc) Handle(ctx context.Context, record slog.Record) error {
	var pcs [10]uintptr
	// skip PC of functions on the stack to get to the actual source
	runtime.Callers(5, pcs[:]) //nolint:gomnd
	r := slog.NewRecord(record.Time, record.Level, record.Message, pcs[0])
	record.Attrs(func(attr slog.Attr) bool {
		r.Add(attr)
		return true
	})
	return s.Handler.Handle(ctx, r)
}

func (s skipPc) WithAttrs(attrs []slog.Attr) slog.Handler {
	return skipPc{Handler: s.Handler.WithAttrs(attrs)}
}

func (s skipPc) WithGroup(name string) slog.Handler {
	return skipPc{Handler: s.Handler.WithGroup(name)}
}
