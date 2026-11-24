package testkit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/router"
)

func previewVite(static http.Handler, mode string) (http.Handler, error) {
	host := "127.0.0.1"
	switch mode {
	case "":
		return static, nil
	case "docker":
		host = "host.docker.internal"
	case "local":
	default:
		return nil, fmt.Errorf("PANEL_TEST_BROWSER_VITE must be local or docker")
	}
	target := url.URL{Scheme: "http", Host: net.JoinHostPort(host, "28742")}
	return previewViteForTarget(static, &target), nil
}

func previewViteForTarget(static http.Handler, target *url.URL) http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 20 * time.Second
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.Host = browserFixtureAddr
			request.Out.Header.Del("Cookie")
			request.Out.Header.Del("Authorization")
		},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			return nil
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		document := router.IsPanelDocumentRoute(path)
		development := path == "/__vite_hmr" || strings.HasPrefix(path, "/src/") || strings.HasPrefix(path, "/@vite/") ||
			strings.HasPrefix(path, "/@id/") || strings.HasPrefix(path, "/@fs/") || strings.HasPrefix(path, "/node_modules/") || path == "/@react-refresh"
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !document && !development {
			static.ServeHTTP(w, r)
			return
		}
		if r.Host != browserFixtureAddr || r.URL.Path == "/__vite_hmr" && r.Header.Get("Origin") != "http://"+browserFixtureAddr {
			http.Error(w, "Invalid preview origin", http.StatusForbidden)
			return
		}
		policy := w.Header().Get("Content-Security-Policy")
		if document {
			devPolicy := strings.Replace(policy, "script-src 'self'", "script-src 'self' 'unsafe-inline'", 1)
			devPolicy = strings.Replace(devPolicy, "connect-src 'self'", "connect-src 'self' ws://"+browserFixtureAddr, 1)
			w.Header().Set("Content-Security-Policy", devPolicy)
		}
		requestProxy := *proxy
		requestProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
			w.Header().Set("Content-Security-Policy", policy)
			static.ServeHTTP(w, r)
		}
		requestProxy.ServeHTTP(w, r)
	})
}

func RunBrowserFixture(t *testing.T) {
	configured := os.Getenv("PANEL_TEST_BROWSER_ADDR")
	if configured == "" {
		t.Skip("set PANEL_TEST_BROWSER_ADDR to enable the fixed-port browser fixture")
	}
	if configured != browserFixtureAddr {
		t.Fatalf("the browser fixture is fixed at %s", browserFixtureAddr)
	}
	address := browserFixtureAddr
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("the browser fixture must bind to IPv4 loopback")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	distDir := "web/dist"
	if info, err := os.Stat(distDir + "/index.html"); err != nil || !info.Mode().IsRegular() {
		if info2, err2 := os.Stat("../../web/dist/index.html"); err2 == nil && info2.Mode().IsRegular() {
			distDir = "../../web/dist"
		} else {
			t.Fatal("build the web bundle first")
		}
	}
	files := router.SPAFileServer(os.DirFS(distDir))
	previewFiles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if router.IsPanelDocumentRoute(r.URL.Path) || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	frontend, err := previewVite(previewFiles, os.Getenv("PANEL_TEST_BROWSER_VITE"))
	if err != nil {
		t.Fatal(err)
	}
	f := setupFixture(t, frontend, "http://"+listener.Addr().String())
	if path := os.Getenv("PANEL_TEST_BROWSER_USAGE_FILE"); path != "" {
		if err := f.loadPreviewUsage(path); err != nil {
			t.Fatal(err)
		}
		f.panel.UsageHub.Build = f.buildPreviewUsageSnapshot
		f.panel.UsageSampleVersion = previewSampleVersion
		t.Logf("loaded %d anonymized usage records (%s)", len(f.usageRows), previewSampleVersion)
	}
	if path := os.Getenv("PANEL_TEST_BROWSER_CATALOG_FILE"); path != "" {
		if err := f.loadPreviewCatalog(path); err != nil {
			t.Fatal(err)
		}
		t.Logf("loaded anonymized preview catalog")
	}
	f.panel.DevAccounts = []core.DevLoginAccount{
		{Label: "普通用户", Email: "user@example.test", Password: fixturePassword},
		{Label: "管理员", Email: "admin@example.test", Password: fixturePassword, OTP: "123456"},
		{Label: "其他用户", Email: "other@example.test", Password: fixturePassword},
	}
	for id := int64(4); id <= maxPreviewUser; id++ {
		if len(f.previewKeys[id]) > 0 {
			f.panel.DevAccounts = append(f.panel.DevAccounts, core.DevLoginAccount{
				Label: fmt.Sprintf("样本用户 %02d", id-1), Email: previewEmail(id), Password: fixturePassword,
			})
		}
	}
	seedPreviewPatrol(t, f.panel)
	previewMux := http.NewServeMux()
	previewMux.Handle("/", f.app)
	previewMux.HandleFunc("GET /v1/usage", f.previewPublicKeyUsage)
	server := &http.Server{Handler: previewMux, ReadHeaderTimeout: 5 * time.Second}
	t.Logf("browser fixture ready at http://%s", listener.Addr())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}

func defaultProtocolForPlatform(plat string) string {
	switch plat {
	case "anthropic":
		return "anthropic-messages"
	case "gemini":
		return "gemini"
	default:
		return "openai-chat"
	}
}

func seedPreviewPatrol(t *testing.T, app *core.Server) {
	t.Helper()
	if app.Database == nil {
		return
	}
	ctx := context.Background()
	if n, err := app.Database.PatrolPolicy.Query().Count(ctx); err != nil || n > 0 {
		return
	}
	now := time.Now()
	groups := []struct {
		id       int64
		name     string
		platform string
		rate     float64
		engine   string
		model    string
		effort   string
		minUsers int
	}{
		{8, "旗舰满血高并发主力通道 · 生产级粘性会话保障组", "openai", 0.30, "lmfpd", "gpt-5.4", "low", 3},
		{9, "Claude Sonnet 研发编码专用直连池", "anthropic", 0.25, "lmfpd", "claude-sonnet-4-6", "medium", 3},
		{10, "特惠低峰弹性推理通道", "openai", 0.12, "gpt-relay-inspector", "gpt-5.4-mini", "low", 5},
	}
	for _, g := range groups {
		_, _ = app.Database.PatrolPolicy.Create().
			SetGroupID(g.id).
			SetGroupName(g.name).
			SetPlatform(g.platform).
			SetRateMultiplier(g.rate).
			SetEnabled(true).
			SetPublicVisible(true).
			SetEngine(g.engine).
			SetModel(g.model).
			SetProtocol(defaultProtocolForPlatform(g.platform)).
			SetEffort(g.effort).
			SetWindowMinutes(15).
			SetMinActiveUsers(g.minUsers).
			SetMinWindowRequests(15).
			SetPeakActiveUsers(15).
			SetIntervalMinutes(30).
			SetPeakIntervalMinutes(15).
			SetRecheckMinutes(5).
			SetDailyTokenLimit(300000).
			SetDailyBudgetCents(200).
			SetNextCheckAt(now.Add(18 * time.Minute).Unix()).
			SetUpdatedAt(now.Unix()).
			Save(ctx)

		for i := 59; i >= 0; i-- {
			ts := now.Add(-time.Duration(i*30) * time.Minute)
			verdict := "ok"
			trigger := "scheduled"
			inTok, outTok, reasonTok, cacheTok, durMs, ttftMs := int64(4280), int64(192), int64(148), int64(3840), int64(14200+(i%7)*950), int64(320+(i%5)*45)
			summary := "数字分布特征与目标模型家族一致，回答命中独立数字 14"
			reply := "Comparing lemon and strawberry candy counts after replacement...\n\n14"
			conf := 0.96
			if i%13 == 5 {
				verdict = "skipped_idle"
				trigger = "idle_skip"
				inTok, outTok, reasonTok, cacheTok, durMs, ttftMs = 0, 0, 0, 0, 0, 0
				summary = "近 15 分钟仅 1 名活跃用户（阈值 3），自动静默免检省 Token"
				reply = ""
			} else if g.id == 10 && i == 11 {
				verdict = "degraded"
				conf = 0.41
				summary = "流式输出特征疑似跨档降级，已触发短周期复检"
			}
			create := app.Database.PatrolRun.Create().
				SetGroupID(g.id).
				SetGroupName(g.name).
				SetTrigger(trigger).
				SetVerdict(verdict).
				SetEngine(g.engine).
				SetModel(g.model).
				SetEffort(g.effort).
				SetActiveUsers(max(1, (i%8)+1)).
				SetWindowRequests(int64((i % 19) + 4)).
				SetDurationMs(durMs).
				SetTtftMs(ttftMs).
				SetInputTokens(inTok).
				SetOutputTokens(outTok).
				SetReasoningTokens(reasonTok).
				SetCachedTokens(cacheTok).
				SetEstimatedMicros(inTok*3 + outTok*15).
				SetSummary(summary).
				SetReplyExcerpt(reply).
				SetCreatedAt(ts)
			if verdict != "skipped_idle" {
				create.SetConfidence(conf).SetPredictedModel(g.model)
			}
			_, _ = create.Save(ctx)
		}
	}
}
