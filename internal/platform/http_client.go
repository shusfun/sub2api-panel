package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ServiceClient struct {
	Base   string
	Client *http.Client
}

func NewClient(base string) ServiceClient {
	return ServiceClient{Base: strings.TrimRight(base, "/"), Client: &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (c ServiceClient) WithTimeout(timeout time.Duration) ServiceClient {
	if c.Client == nil {
		c = NewClient(c.Base)
	}
	clone := *c.Client
	clone.Timeout = timeout
	c.Client = &clone
	return c
}

func (c ServiceClient) Open(ctx context.Context, method, path, token, key, operation string, data any) (*http.Response, error) {
	var body io.Reader
	var contentType string
	if data != nil {
		value, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(value)
		contentType = "application/json"
	}
	return c.OpenBody(ctx, method, path, token, key, operation, body, contentType)
}

func (c ServiceClient) OpenBody(ctx context.Context, method, path, token, key, operation string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if operation != "" {
		req.Header.Set("Idempotency-Key", operation)
	}
	info := Meta(ctx)
	if info.UA != "" {
		req.Header.Set("User-Agent", info.UA)
	}
	if info.Referer != "" {
		req.Header.Set("Referer", info.Referer)
	}
	span := RandomHex(8)
	if info.Trace != "" {
		req.Header.Set("traceparent", "00-"+info.Trace+"-"+span+"-01")
		req.Header.Set("X-Request-ID", info.Trace)
	}
	start := time.Now()
	res, err := c.Client.Do(req)
	status := 0
	if res != nil {
		status = res.StatusCode
	}
	route, _, _ := strings.Cut(path, "?")
	slog.Info("service request", "trace_id", info.Trace, "span_id", span, "parent_span_id", info.Span, "method", method, "route", route, "status", status, "elapsed_ms", time.Since(start).Milliseconds())
	return res, err
}

func Upstream[T any](ctx context.Context, c ServiceClient, method, path, token, key, operation string, input any) (T, error) {
	var zero T
	res, err := c.Open(ctx, method, "/api/v1"+path, token, key, operation, input)
	if err != nil {
		return zero, err
	}
	defer res.Body.Close()
	return ReadUpstream[T](res)
}

func ReadUpstream[T any](res *http.Response) (T, error) {
	var zero T
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return zero, UpstreamStatus{Status: res.StatusCode}
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	if dec.Decode(&envelope) != nil || dec.Decode(new(any)) != io.EOF ||
		envelope.Code == nil || *envelope.Code != 0 {
		return zero, Fail(502, "invalid_upstream")
	}
	if _, empty := any(zero).(struct{}); empty {
		return zero, nil
	}
	if bytes.Equal(envelope.Data, []byte("null")) || json.Unmarshal(envelope.Data, &zero) != nil {
		return zero, Fail(502, "invalid_upstream")
	}
	return zero, nil
}

func CheckOrigin(raw string, httpsOnly bool) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Hostname() != "" && u.User == nil && u.Path == "" &&
		u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery &&
		(u.Scheme == "https" || !httpsOnly && u.Scheme == "http")
}

func ExternalLink(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return nil, false
	}
	return u, true
}

func ValidEmail(value string) bool {
	at := strings.LastIndex(value, "@")
	return at > 0 && at < len(value)-3 && len(value) <= 320 && !strings.ContainsAny(value, " \t\r\n")
}

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func FirstNonNil[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
