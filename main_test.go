package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"sub2api-panel/internal/server/router"
	"sub2api-panel/internal/testkit"
)

func TestBrowserFixture(t *testing.T) {
	testkit.RunBrowserFixture(t)
}

func TestOfficialCompatibility(t *testing.T) {
	testkit.RunOfficialCompatibility(t)
}

func TestArchGuard(t *testing.T) {
	// 1. 根目录只允许保留 4 个 Go 入口文件，禁止根部出现临时 -*.md 杂物文件
	allowedRootGo := map[string]bool{
		"main.go":          true,
		"embed_dev.go":     true,
		"embed_release.go": true,
		"main_test.go":     true,
	}
	rootEntries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range rootEntries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(name, ".go") && !allowedRootGo[name] {
			t.Errorf("unexpected root Go file %q; root may only keep main.go, embed_dev.go, embed_release.go, main_test.go", name)
		}
		if strings.HasPrefix(name, "-") && strings.HasSuffix(name, ".md") {
			t.Errorf("unexpected root clutter file %q", name)
		}
	}

	// 2. 递归扫描 internal/**/*.go（跳过 *_test.go）校验分层、安全边界与单文件行数
	fset := token.NewFileSet()
	allowedSQLFiles := map[string]bool{
		"internal/platform/database.go":            true,
		"internal/server/observe/official_stats.go": true,
	}
	viewTypeSuffix := regexp.MustCompile(`(View|Row|Summary|Response)$`)
	const maxLines = 600

	err = filepath.WalkDir("internal", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := bytes.Count(raw, []byte{'\n'})
		if len(raw) > 0 && raw[len(raw)-1] != '\n' {
			lines++
		}
		if lines > maxLines {
			t.Errorf("%s has %d lines; non-test Go files must be <= %d lines", rel, lines, maxLines)
		}

		fileAST, err := parser.ParseFile(fset, path, raw, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		inServer := strings.HasPrefix(rel, "internal/server/")
		for _, imp := range fileAST.Imports {
			pkg := strings.Trim(imp.Path.Value, `"`)
			if inServer && pkg == "sub2api-panel/internal/issuer" {
				t.Errorf("%s imports internal/issuer; HTTP server must only talk to issuer via loopback client", rel)
			}
			if pkg == "database/sql" && !allowedSQLFiles[rel] {
				t.Errorf("%s imports database/sql directly; all queries must go through Ent", rel)
			}
			if strings.Contains(strings.ToLower(pkg), "sqlite") {
				t.Errorf("%s imports forbidden sqlite package %s", rel, pkg)
			}
		}

		ast.Inspect(fileAST, func(n ast.Node) bool {
			// 禁止 decodeBody[...] / DecodeBody[...] 面板输入结构体使用 totp_code 异名字段
			if call, ok := n.(*ast.CallExpr); ok {
				if idx, ok := call.Fun.(*ast.IndexExpr); ok && isDecodeBodyFunc(idx.X) {
					if st, ok := idx.Index.(*ast.StructType); ok {
						for _, field := range st.Fields.List {
							if field.Tag != nil && strings.Contains(field.Tag.Value, `"totp_code`) {
								pos := fset.Position(field.Pos())
								t.Errorf("%s:%d DecodeBody uses legacy totp_code tag; panel endpoints must use totp", rel, pos.Line)
							}
						}
					}
				}
			}

			// 禁止对外投影 View/Row/Summary/Response 结构体暴露裸 map[...]any
			if ts, ok := n.(*ast.TypeSpec); ok && viewTypeSuffix.MatchString(ts.Name.Name) {
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						if mt, ok := field.Type.(*ast.MapType); ok {
							if valIdent, ok := mt.Value.(*ast.Ident); ok && valIdent.Name == "any" {
								if rel == "internal/server/pool/account_data.go" && ts.Name.Name == "accountBundleRow" {
									// accountBundleRow 导出时已置空 map，保留官方导入包结构
									continue
								}
								pos := fset.Position(field.Pos())
								t.Errorf("%s:%d projection struct %s exposes untyped map[...]any", rel, pos.Line, ts.Name.Name)
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
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

	// 4. 校验前端 web/src/app/route.ts 声明的全部文档路径均被 router.IsPanelDocumentRoute 覆盖
	routeFile := filepath.Join("web", "src", "app", "route.ts")
	routeBytes, err := os.ReadFile(routeFile)
	if err != nil {
		t.Fatal(err)
	}
	pathLiteral := regexp.MustCompile(`:\s*"(/[^"]*)"`)
	matches := pathLiteral.FindAllStringSubmatch(string(routeBytes), -1)
	if len(matches) == 0 {
		t.Fatalf("no route literals found in %s", routeFile)
	}
	for _, match := range matches {
		routePath := match[1]
		if !router.IsPanelDocumentRoute(routePath) {
			t.Errorf("frontend route %q in %s is not registered in router.IsPanelDocumentRoute", routePath, routeFile)
		}
	}
	for _, extra := range []string{
		"/_ui",
		"/oauth/mcp/authorize",
		"/payment/result",
		"/admin/accounts/12",
		"/admin/groups/5",
		"/admin/proxies/3",
		"/admin/users/7",
		"/keys/9",
		"/custom/status-board",
	} {
		if !router.IsPanelDocumentRoute(extra) {
			t.Errorf("expected document route %q to be served by SPA fallback", extra)
		}
	}
	for _, removed := range []string{"/channels/available", "/channels/monitor", "/admin/promo-codes"} {
		if router.IsPanelDocumentRoute(removed) {
			t.Errorf("removed legacy route %q must not be served by router.IsPanelDocumentRoute", removed)
		}
	}
}

func isDecodeBodyFunc(expr ast.Expr) bool {
	switch fn := expr.(type) {
	case *ast.Ident:
		return fn.Name == "decodeBody" || fn.Name == "DecodeBody"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "decodeBody" || fn.Sel.Name == "DecodeBody"
	default:
		return false
	}
}
