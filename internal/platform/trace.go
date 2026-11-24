package platform

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

var (
	TraceHex          = regexp.MustCompile(`^[a-f0-9]{32}$`)
	SpanHex           = regexp.MustCompile(`^00-([a-f0-9]{32})-([a-f0-9]{16})-01$`)
	OperationPattern  = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
	OTPPattern        = regexp.MustCompile(`^[0-9]{6}$`)
	CustomSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)
)

var buildCommit atomic.Value

func init() {
	buildCommit.Store("unknown")
}

func SetBuildCommit(c string) {
	if c != "" {
		buildCommit.Store(c)
	}
}

func BuildCommit() string {
	if v, ok := buildCommit.Load().(string); ok && v != "" {
		return v
	}
	return "unknown"
}

type CallMeta struct {
	Trace   string
	Span    string
	UA      string
	Referer string
}

type metaKey struct{}

func RandomHex(size int) string { return hex.EncodeToString(RandomBytes(size)) }

func RandomBytes(size int) []byte {
	data := make([]byte, size)
	_, _ = rand.Read(data)
	return data
}

func RandomToken() string { return base64.RawURLEncoding.EncodeToString(RandomBytes(32)) }

func WithMeta(ctx context.Context, m CallMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

func Meta(ctx context.Context) CallMeta {
	value, _ := ctx.Value(metaKey{}).(CallMeta)
	return value
}

func TraceRequest(r *http.Request, w http.ResponseWriter, name string) (*http.Request, func()) {
	info := CallMeta{Trace: RandomHex(16), Span: RandomHex(8), UA: r.UserAgent()}
	if parent := SpanHex.FindStringSubmatch(r.Header.Get("traceparent")); len(parent) == 3 && parent[1] != strings.Repeat("0", 32) && parent[2] != strings.Repeat("0", 16) {
		info.Trace = parent[1]
	}
	start := time.Now()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", info.Trace)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return r.WithContext(WithMeta(r.Context(), info)), func() {
		slog.Info(name, "trace_id", info.Trace, "span_id", info.Span, "elapsed_ms", time.Since(start).Milliseconds())
	}
}

func callerLocation(skip int) (file string, fn string, line int) {
	pc, rawFile, lineNo, ok := runtime.Caller(skip)
	if !ok {
		return "", "", 0
	}
	file = filepath.ToSlash(rawFile)
	if idx := strings.LastIndex(file, "/sub2api-panel/"); idx >= 0 {
		file = file[idx+len("/sub2api-panel/"):]
	} else {
		file = filepath.Base(file)
	}
	if f := runtime.FuncForPC(pc); f != nil {
		fn = f.Name()
		if idx := strings.LastIndex(fn, "/"); idx >= 0 {
			fn = fn[idx+1:]
		}
	}
	return file, fn, lineNo
}

func StartFlow(ctx context.Context, flow string, attrs ...any) (context.Context, func()) {
	current := Meta(ctx)
	parent := current.Span
	if current.Trace == "" {
		current.Trace = RandomHex(16)
	}
	current.Span = RandomHex(8)
	file, fn, line := callerLocation(2)
	start := time.Now()
	next := WithMeta(ctx, current)
	return next, func() {
		base := []any{
			"flow", flow,
			"trace_id", current.Trace,
			"span_id", current.Span,
			"parent_span_id", parent,
			"code.file.path", file,
			"code.function.name", fn,
			"code.line.number", line,
			"vcs.ref.head.revision", BuildCommit(),
			"elapsed_ms", time.Since(start).Milliseconds(),
		}
		slog.Info("business flow", append(base, attrs...)...)
	}
}

func BackgroundFlow(ctx context.Context, flow string, attrs ...any) (context.Context, func()) {
	base := Meta(ctx)
	if base.Trace == "" {
		ctx = WithMeta(ctx, CallMeta{Trace: RandomHex(16)})
	}
	file, fn, line := callerLocation(2)
	current := Meta(ctx)
	current.Span = RandomHex(8)
	start := time.Now()
	next := WithMeta(ctx, current)
	return next, func() {
		fields := []any{
			"flow", flow,
			"trace_id", current.Trace,
			"span_id", current.Span,
			"code.file.path", file,
			"code.function.name", fn,
			"code.line.number", line,
			"vcs.ref.head.revision", BuildCommit(),
			"elapsed_ms", time.Since(start).Milliseconds(),
		}
		slog.Info("background flow", append(fields, attrs...)...)
	}
}
