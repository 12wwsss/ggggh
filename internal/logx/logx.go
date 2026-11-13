// Package logx 提供结构化日志与请求级 trace id。
package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

// New 构造 JSON 结构化日志器。
func New(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv})
	return slog.New(h)
}

// WithTraceID 把 trace id 放进 context。
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// TraceID 取出 trace id，没有则返回空串。
func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
