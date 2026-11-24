package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
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
)

func TestOfficialPluginUploadPreservesSignedBytes(t *testing.T) {
	const uploadPath = "/admin/plugins/upstream/upload"
	content := strings.Repeat("package bytes\x00", maxBody/8)
	var uploadCalls atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/me":
			role := "admin"
			if r.Header.Get("Authorization") == "Bearer viewer" {
				role = "user"
			}
			officialSuccess(w, identity{ID: 1, Role: role})
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
	p := &panel{origin: fixtureOrigin, upstream: newClient(up.URL), sessions: newSessions()}
	app := p.routes(http.NotFoundHandler())
	newRequest := func(token string, tamper bool) *httptest.ResponseRecorder {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		id, err := p.sessions.create(time.Now(), &key.PublicKey, identity{ID: 1, Role: "admin"}, credentials{Access: token})
		if err != nil {
			t.Fatal(err)
		}
		b := fixtureBrowser{t: t, app: app, key: key, csrf: p.sessions.active[sha256.Sum256([]byte(id))].csrf, cookie: &http.Cookie{Name: cookieName, Value: id}}
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
	app := panel{upstream: newClient(upstreamServer.URL)}
	ctx := context.WithValue(context.Background(), metaKey{}, callMeta{Trace: randomHex(16), Span: randomHex(8)})
	check := func(page string) pageResult[accountAdminView] {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/accounts?sort_by=activity&page="+page, nil).WithContext(ctx)
		out, err := app.accountActivityPage(req, "test")
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

func activityIDs(page pageResult[accountAdminView]) []int64 {
	ids := make([]int64, 0, len(page.Items))
	for _, row := range page.Items {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestActivityTieBreak(t *testing.T) {
	when := "2026-11-23T06:00:00+08:00"
	rows := []accountAdminView{{ID: 4, Priority: 1, LastUsedAt: &when}, {ID: 2, Priority: 1, LastUsedAt: &when}, {ID: 3, Priority: 0, LastUsedAt: &when}, {ID: 1, Priority: 0}}
	sortAccountsByActivity(rows)
	if got := activityIDs(pageResult[accountAdminView]{Items: rows}); !reflect.DeepEqual(got, []int64{3, 2, 4, 1}) {
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
		out, err := readBalanceResponse(strings.NewReader(tc.body), tc.source)
		if err != nil || out.Kind != tc.kind || out.Balance != tc.amount {
			t.Fatalf("%s: %+v %v", tc.source, out, err)
		}
	}
	if _, err := readBalanceResponse(strings.NewReader(`{"mode":"quota_limited"}`), "sub2api"); err == nil {
		t.Fatal("missing amount accepted")
	}
	if _, err := balanceEndpoint(balanceConfig{Provider: "sub2api", BaseURL: "https://127.0.0.1"}); err == nil {
		t.Fatal("loopback endpoint accepted")
	}
	if _, err := balanceEndpoint(balanceConfig{Provider: "sub2api", BaseURL: "https://[::ffff:127.0.0.1]"}); err == nil {
		t.Fatal("mapped loopback endpoint accepted")
	}
}

func TestHistoryDocumentRouting(t *testing.T) {
	handler := spaFileServer(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}})
	for route, status := range map[string]int{"/": 200, "/login": 200, "/auth/callback": 200, "/admin/accounts/14?sort=activity": 200, "/custom/docs": 200, "/keys/7": 200, "/api/nonexistent": 404, "/assets/no.js": 404, "/admin/accounts/not-id": 404} {
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
	input := []customMenuItem{{ID: "one", Label: "first", URL: "https://example.test", Visibility: "user"}, {ID: "two", Label: "second", URL: "https://example.test", Visibility: "admin"}, {ID: "bad", Label: "other", URL: "javascript:alert(1)", Visibility: "user"}}
	user := projectCustomMenu(input, "user")
	if len(user) != 1 || user[0].ID != "one" {
		t.Fatal(user)
	}
	admin := projectCustomMenu(input, "admin")
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
	manifest := updateManifest{Version: "v1.0.0", SHA256: "00"}
	if err := installUpdate(candidate, target, manifest); err == nil {
		t.Fatal("invalid checksum accepted")
	}
	manifest.SHA256 = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := installUpdate(candidate, target, manifest); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "abc" {
		t.Fatal("candidate was not installed")
	}
	if err := rollbackUpdate(target); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "old" {
		t.Fatal("backup was not restored")
	}
}
