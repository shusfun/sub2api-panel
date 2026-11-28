package panelupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"sub2api-panel/internal/platform"
)

type Job struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Phase     string    `json:"phase"`
	Failure   string    `json:"failure,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Status struct {
	Configured bool `json:"configured"`
	Job        *Job `json:"job,omitempty"`
}

type Check struct {
	Release *ReleaseInfo `json:"release,omitempty"`
}

type Request struct {
	Version string `json:"version"`
	Actor   int64  `json:"actor"`
}

type Client struct{ client *http.Client }

func NewClient(socket string) *Client {
	if socket == "" {
		return nil
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	return &Client{client: &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var out Status
	if c == nil {
		return out, nil
	}
	err := c.call(ctx, http.MethodGet, "/status", nil, &out)
	return out, err
}

func (c *Client) Check(ctx context.Context) (Check, error) {
	var out Check
	if c == nil {
		return out, platform.Fail(503, "panel_update_unavailable")
	}
	err := c.call(ctx, http.MethodGet, "/release", nil, &out)
	return out, err
}

func (c *Client) Submit(ctx context.Context, input Request) (Job, error) {
	var out Job
	if c == nil {
		return out, platform.Fail(503, "panel_update_unavailable")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return out, err
	}
	err = c.call(ctx, http.MethodPost, "/jobs", bytes.NewReader(body), &out)
	return out, err
}

func (c *Client) call(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, "http://updater"+path, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	meta := platform.Meta(ctx)
	if platform.TraceHex.MatchString(meta.Trace) && len(meta.Span) == 16 {
		req.Header.Set("traceparent", "00-"+meta.Trace+"-"+meta.Span+"-01")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return platform.Fail(503, "panel_update_unavailable")
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusConflict:
		return platform.Fail(409, "panel_update_conflict")
	case http.StatusBadRequest:
		return platform.Fail(400, "invalid_input")
	case http.StatusBadGateway:
		return platform.Fail(502, "panel_update_check_failed")
	case http.StatusOK, http.StatusAccepted:
		if err := readStrict(res.Body, 128<<10, out); err != nil {
			return platform.Fail(502, "panel_update_check_failed")
		}
		return nil
	default:
		return platform.Fail(503, "panel_update_unavailable")
	}
}
