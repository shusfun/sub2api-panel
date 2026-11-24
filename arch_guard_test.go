package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestArchGuard(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	allowedSQLFiles := map[string]bool{
		"database.go":       true,
		"official_stats.go": true,
	}
	viewTypeSuffix := regexp.MustCompile(`(View|Row|Summary|Response)$`)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fileAST, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range fileAST.Imports {
			pkg := strings.Trim(imp.Path.Value, `"`)
			if pkg == "database/sql" && !allowedSQLFiles[name] {
				t.Errorf("%s imports database/sql directly; all queries must go through Ent", name)
			}
			if strings.Contains(strings.ToLower(pkg), "sqlite") {
				t.Errorf("%s imports forbidden sqlite package %s", name, pkg)
			}
		}

		ast.Inspect(fileAST, func(n ast.Node) bool {
			// 1. 禁止 decodeBody[...] 面板输入结构体使用 totp_code 异名字段
			if call, ok := n.(*ast.CallExpr); ok {
				if idx, ok := call.Fun.(*ast.IndexExpr); ok {
					if ident, ok := idx.X.(*ast.Ident); ok && ident.Name == "decodeBody" {
						if st, ok := idx.Index.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								if field.Tag != nil && strings.Contains(field.Tag.Value, `"totp_code`) {
									pos := fset.Position(field.Pos())
									t.Errorf("%s:%d decodeBody uses legacy totp_code tag; panel endpoints must use totp", pos.Filename, pos.Line)
								}
							}
						}
					}
				}
			}

			// 2. 禁止对外投影 View/Row/Summary 结构体暴露裸 map[string]any
			if ts, ok := n.(*ast.TypeSpec); ok && viewTypeSuffix.MatchString(ts.Name.Name) {
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						if mt, ok := field.Type.(*ast.MapType); ok {
							if valIdent, ok := mt.Value.(*ast.Ident); ok && valIdent.Name == "any" {
								if name == "account_data.go" && ts.Name.Name == "accountBundleRow" {
									// accountBundleRow 导出时已置空 map，保留官方导入包结构
									continue
								}
								pos := fset.Position(field.Pos())
								t.Errorf("%s:%d projection struct %s exposes untyped map[...]any", pos.Filename, pos.Line, ts.Name.Name)
							}
						}
					}
				}
			}
			return true
		})
	}

	// 3. 校验 ent/schema/*.go 均具备完整的 6 行数据模型说明头及有效 ADR 路径
	requiredHeaders := []string{
		"// 变更类型:",
		"// 权威来源:",
		"// 查询方式:",
		"// 保留期限:",
		"// 旧数据:",
		"// 关联决策:",
	}
	schemaEntries, err := os.ReadDir(filepath.Join("ent", "schema"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range schemaEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		full := filepath.Join("ent", "schema", entry.Name())
		raw, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		head := string(raw)
		if idx := strings.Index(head, "\npackage "); idx >= 0 {
			head = head[:idx]
		}
		for _, prefix := range requiredHeaders {
			if !strings.Contains(head, prefix) {
				t.Errorf("%s missing required schema header %q", full, prefix)
			}
		}
		for _, line := range strings.Split(head, "\n") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// 关联决策:"); ok {
				adr := strings.TrimSpace(rest)
				if _, statErr := os.Stat(filepath.FromSlash(adr)); statErr != nil {
					t.Errorf("%s references missing decision record %q", full, adr)
				}
			}
		}
	}

	// 4. 校验前端 route.ts 声明的全部文档路径均被 panel.go isPanelDocumentRoute 覆盖
	routeBytes, err := os.ReadFile(filepath.Join("web", "src", "route.ts"))
	if err != nil {
		t.Fatal(err)
	}
	pathLiteral := regexp.MustCompile(`:\s*"(/[^"]*)"`)
	for _, match := range pathLiteral.FindAllStringSubmatch(string(routeBytes), -1) {
		routePath := match[1]
		if !isPanelDocumentRoute(routePath) {
			t.Errorf("frontend route %q in web/src/route.ts is not registered in isPanelDocumentRoute", routePath)
		}
	}
	for _, extra := range []string{"/_ui", "/oauth/mcp/authorize", "/payment/result", "/admin/accounts/12", "/admin/users/7", "/keys/9", "/custom/status-board"} {
		if !isPanelDocumentRoute(extra) {
			t.Errorf("expected document route %q to be served by SPA fallback", extra)
		}
	}
}

func TestFieldErrorAndConflictContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rec := httptest.NewRecorder()
	req, end := traceRequest(req, rec, "POST /api/test")
	end()
	writeError(rec, req, failFields(map[string]string{"name": "名称不能为空", "allowed_ips": "CIDR 格式有误"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var out problemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != "invalid_input" || out.Fields["name"] != "名称不能为空" || out.Fields["allowed_ips"] != "CIDR 格式有误" || out.ID == "" {
		t.Fatalf("unexpected field error payload: %+v", out)
	}

	conflictErr := translateUpstream(upstreamStatus{status: http.StatusConflict})
	var prob *problem
	if conflictErr == nil || conflictErr.Error() != "conflict" {
		t.Fatalf("expected 409 upstream status to map to conflict, got %v", conflictErr)
	}
	_ = prob
}

func TestOfficialPluginConfigRedactionAndMerge(t *testing.T) {
	stored := map[string]json.RawMessage{
		"api_key":     json.RawMessage(`"sk-super-secret"`),
		"enabled":     json.RawMessage(`true`),
		"max_retries": json.RawMessage(`3`),
		"endpoint":    json.RawMessage(`"https://relay.example.test"`),
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/settings":
			officialSuccess(w, map[string]any{"step_up_enabled": false})
		case "/api/v1/admin/plugins/1/config":
			if r.Method == http.MethodPut {
				var next map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				stored = next
			}
			writeJSON(w, http.StatusOK, stored)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer up.Close()

	p := &panel{origin: fixtureOrigin, upstream: newClient(up.URL)}
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/plugins/upstream/1/config", nil)
	getReq.SetPathValue("id", "1")
	getRec := httptest.NewRecorder()
	if err := p.officialPluginConfig(getRec, getReq, sessionAuth{user: identity{ID: 1, Role: "admin"}, tokens: credentials{Access: "op"}}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte("sk-super-secret")) {
		t.Fatalf("plugin secret leaked in GET response: %s", getRec.Body.String())
	}
	var view officialPluginConfigView
	if err := json.Unmarshal(getRec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Fields) != 4 || view.Fields[0].Key != "api_key" || view.Fields[0].Kind != "secret" || !view.Fields[0].Configured {
		t.Fatalf("unexpected classified plugin fields: %+v", view.Fields)
	}

	putBody := `{"totp":"","config":{"enabled":false,"max_retries":5,"endpoint":"https://new.example.test"},"preserve_secrets":["api_key"]}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/admin/plugins/upstream/1/config", strings.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.SetPathValue("id", "1")
	putRec := httptest.NewRecorder()
	if err := p.officialPluginConfig(putRec, putReq, sessionAuth{user: identity{ID: 1, Role: "admin"}, tokens: credentials{Access: "op"}}); err != nil {
		t.Fatal(err)
	}
	if string(stored["api_key"]) != `"sk-super-secret"` || string(stored["max_retries"]) != "5" {
		t.Fatalf("expected secret to be preserved and max_retries updated, got %s", stored)
	}
}
