package testkit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"sub2api-panel/internal/cli"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/pool"
	"sub2api-panel/internal/server/router"
)

func TestOfficialPluginUploadPreservesSignedBytes(t *testing.T) {
	const uploadPath = "/admin/plugins/upstream/upload"
	content := strings.Repeat("package bytes\x00", platform.MaxBody/8)
	var uploadCalls atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/me":
			role := "admin"
			if r.Header.Get("Authorization") == "Bearer viewer" {
				role = "user"
			}
			officialSuccess(w, platform.Identity{ID: 1, Role: role})
		case "/api/v1/admin/plugins/upload":
			uploadCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer operator" {
				t.Error("official upload did not use the operator session")
			}
			reader, err := r.MultipartReader()
			if err != nil {
				t.Error("official upload is not multipart")
				return
			}
			part, err := reader.NextPart()
			if err != nil || part.FormName() != "plugin" || !strings.HasSuffix(part.FileName(), ".s2plugin") {
				t.Error("official upload file part is invalid")
				return
			}
			value, err := io.ReadAll(part)
			if err != nil || string(value) != content {
				t.Error("multipart package differs from signed request")
			}
			if _, err := reader.NextPart(); err != io.EOF {
				t.Error("multipart upload is missing its final delimiter")
			}
			officialSuccess(w, map[string]any{"id": 1, "name": "test", "manifest_secret": fixtureLeak})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer up.Close()
	p := &core.Server{Origin: fixtureOrigin, Upstream: platform.NewClient(up.URL), Sessions: platform.NewSessions()}
	router.WireHubs(p)
	app := router.NewHandler(p, http.NotFoundHandler())
	newRequest := func(token string, tamper bool) *httptest.ResponseRecorder {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		id, err := p.Sessions.Create(time.Now(), &key.PublicKey, platform.Identity{ID: 1, Role: "admin"}, platform.Credentials{Access: token})
		if err != nil {
			t.Fatal(err)
		}
		b := fixtureBrowser{t: t, app: app, key: key, csrf: p.Sessions.CSRFFor(id), cookie: &http.Cookie{Name: platform.CookieName, Value: id}}
		r := b.signRequest(http.MethodPost, uploadPath, content)
		r.Header.Set("Content-Type", "application/octet-stream")
		if tamper {
			r.Body = io.NopCloser(strings.NewReader(content + "extra"))
		}
		return b.send(r)
	}
	checkResponse(t, newRequest("viewer", false), http.StatusForbidden)
	checkResponse(t, newRequest("operator", true), http.StatusUnauthorized)
	checkResponse(t, newRequest("operator", false), http.StatusCreated)
	if uploadCalls.Load() != 1 {
		t.Fatal("only the signed administrator request should have reached the official upload")
	}
}

func TestActivitySortAcrossPages(t *testing.T) {
	live := true
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/accounts":
			var accounts []map[string]any
			for id := 1; id <= 30; id++ {
				accounts = append(accounts, map[string]any{"id": id, "name": fmt.Sprintf("account-%d", id), "platform": "openai", "type": "apikey", "status": "active", "schedulable": true, "last_used_at": fmt.Sprintf("2026-11-23T00:%02d:00Z", 31-id)})
			}
			officialSuccess(w, map[string]any{"items": accounts, "total": 30})
		case "/api/v1/admin/ops/concurrency":
			if !live {
				w.WriteHeader(503)
				return
			}
			officialSuccess(w, map[string]any{"account": map[string]any{"30": map[string]any{"account_id": 30, "current_in_use": 1}, "2": map[string]any{"account_id": 2, "current_in_use": 0}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstreamServer.Close()
	app := &core.Server{Upstream: platform.NewClient(upstreamServer.URL)}
	ctx := platform.WithMeta(context.Background(), platform.CallMeta{Trace: platform.RandomHex(16), Span: platform.RandomHex(8)})
	check := func(page string) platform.PageResult[pool.AccountAdminView] {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/accounts?sort_by=activity&page="+page, nil).WithContext(ctx)
		out, err := pool.AccountActivityPage(app, req, "test")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := check("1")
	if len(first.Items) != 20 || first.Pages != 2 || first.Items[0].ID != 30 || first.Items[0].ActivityKnown == nil || !*first.Items[0].ActivityKnown || first.Items[1].ID != 1 {
		t.Fatalf("unexpected page 1: %v", activityIDs(first))
	}
	second := check("2")
	if len(second.Items) != 10 || second.Items[0].ID != 20 || second.More {
		t.Fatalf("unexpected page 2: %v", activityIDs(second))
	}
	live = false
	first = check("1")
	if first.Items[0].ID != 1 || first.Items[0].ActivityKnown == nil || *first.Items[0].ActivityKnown {
		t.Fatalf("unexpected fallback order: %v", activityIDs(first))
	}
}

func activityIDs(page platform.PageResult[pool.AccountAdminView]) []int64 {
	ids := make([]int64, 0, len(page.Items))
	for _, row := range page.Items {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestActivityTieBreak(t *testing.T) {
	when := "2026-11-23T06:00:00+08:00"
	rows := []pool.AccountAdminView{{ID: 4, Priority: 1, LastUsedAt: &when}, {ID: 2, Priority: 1, LastUsedAt: &when}, {ID: 3, Priority: 0, LastUsedAt: &when}, {ID: 1, Priority: 0}}
	pool.SortAccountsByActivity(rows)
	if got := activityIDs(platform.PageResult[pool.AccountAdminView]{Items: rows}); !reflect.DeepEqual(got, []int64{3, 2, 4, 1}) {
		t.Fatal(got)
	}
}

func TestBalanceSourceClassification(t *testing.T) {
	cases := []struct {
		source, body, kind string
		amount             float64
	}{
		{"sub2api", `{"mode":"unrestricted","balance":0,"remaining":0}`, "account", 0},
		{"sub2api", `{"mode":"quota_limited","remaining":7}`, "key_quota", 7},
		{"sub2api", `{"mode":"unrestricted","planName":"test","remaining":3}`, "subscription", 3},
		{"deepseek", `{"balance_infos":[{"currency":"CNY","total_balance":"0"}]}`, "account", 0},
		{"kimi", `{"data":{"available_balance":1}}`, "account", 1},
	}
	for _, tc := range cases {
		out, err := core.ReadBalanceResponse(strings.NewReader(tc.body), tc.source)
		if err != nil || out.Kind != tc.kind || out.Balance != tc.amount {
			t.Fatalf("%s: %+v %v", tc.source, out, err)
		}
	}
	if _, err := core.ReadBalanceResponse(strings.NewReader(`{"mode":"quota_limited"}`), "sub2api"); err == nil {
		t.Fatal("missing amount accepted")
	}
	if _, err := core.BalanceEndpoint(core.BalanceConfig{Provider: "sub2api", BaseURL: "https://127.0.0.1"}); err == nil {
		t.Fatal("loopback endpoint accepted")
	}
	if _, err := core.BalanceEndpoint(core.BalanceConfig{Provider: "sub2api", BaseURL: "https://[::ffff:127.0.0.1]"}); err == nil {
		t.Fatal("mapped loopback endpoint accepted")
	}
}

func TestHistoryDocumentRouting(t *testing.T) {
	handler := router.SPAFileServer(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}})
	cases := map[string]int{
		"/":                               200,
		"/login":                          200,
		"/auth/callback":                  200,
		"/admin/accounts/14?sort=activity": 200,
		"/custom/docs":                    200,
		"/keys/7":                         200,
		"/api/nonexistent":                404,
		"/assets/no.js":                   404,
		"/admin/accounts/not-id":          404,
		"/channels/available":             404,
		"/channels/monitor":               404,
		"/admin/promo-codes":              404,
	}
	for route, status := range cases {
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, route, nil))
		if out.Code != status {
			t.Errorf("%s: got %d, want %d", route, out.Code, status)
		}
	}
}

func TestHostedCheckoutUsesTrustedPanelReturn(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	f.preview = &previewCatalog{}
	user := browserLogin(t, f.app, "user")
	other := browserLogin(t, f.app, "other")
	user.expect("GET", "/payment/checkout", "", 200)
	created := user.expect("POST", "/payment/orders", `{"amount":10,"order_type":"balance","payment_type":"alipay"}`, 200)
	id := int64(created["order_id"].(float64))
	path := fmt.Sprintf("/payment/orders/%d", id)
	other.expect("GET", path, "", 404)
	other.expect("POST", path+"/cancel", "", 404)
	user.expect("GET", path, "", 200)
	user.expect("POST", path+"/cancel", "", 200)
	if got := user.expect("GET", path, "", 200); got["status"] != "CANCELLED" {
		t.Fatal("cancelled order status did not persist")
	}
	user.expect("POST", "/payment/orders", `{"amount":10,"order_type":"balance","payment_type":"alipay","return_url":"https://external.example/payment/result"}`, 400)
	resume := user.expect("POST", "/public/payment/result", `{"resume_token":"preview-payment-return"}`, 200)
	if resume["id"] != float64(id) || resume["status"] != "CANCELLED" {
		t.Fatal("callback did not resolve the known test order")
	}
}

func TestCustomMenusAreScoped(t *testing.T) {
	input := []core.CustomMenuItem{{ID: "one", Label: "first", URL: "https://example.test", Visibility: "user"}, {ID: "two", Label: "second", URL: "https://example.test", Visibility: "admin"}, {ID: "bad", Label: "other", URL: "javascript:alert(1)", Visibility: "user"}}
	user := core.ProjectCustomMenu(input, "user")
	if len(user) != 1 || user[0].ID != "one" {
		t.Fatal(user)
	}
	admin := core.ProjectCustomMenu(input, "admin")
	if len(admin) != 1 || admin[0].ID != "two" {
		t.Fatal(admin)
	}
	out, err := json.Marshal(user)
	if err != nil || strings.Contains(string(out), "admin") {
		t.Fatalf("unexpected public menu projection")
	}
}

func TestLocalBinaryUpdateAndRollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "panel")
	candidate := filepath.Join(dir, "candidate")
	if err := os.WriteFile(target, []byte("old"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := cli.UpdateManifest{Version: "v1.0.0", SHA256: "00"}
	if err := cli.InstallUpdate(candidate, target, manifest); err == nil {
		t.Fatal("invalid checksum accepted")
	}
	manifest.SHA256 = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := cli.InstallUpdate(candidate, target, manifest); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "abc" {
		t.Fatal("candidate was not installed")
	}
	if err := cli.RollbackUpdate(target); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "old" {
		t.Fatal("backup was not restored")
	}
}
