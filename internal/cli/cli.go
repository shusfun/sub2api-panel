package cli

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"sub2api-panel/internal/issuer"
	"sub2api-panel/internal/panelupdate"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
	"sub2api-panel/internal/server/router"
	"sub2api-panel/internal/server/system"
)

const cliHelp = `Sub2API 管理面板

用法：sub2api-panel <命令>

  serve             启动面板
  issuer            启动发码进程
  migrate           执行 PostgreSQL 迁移
  --version         显示版本和构建信息
  update check      核对下载后的候选二进制
  update install    安装已经校验的候选二进制
  update rollback   恢复上次安装前的二进制
  update executor   运行 systemd 更新执行器

使用 update <命令> --help 查看参数。
`

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		slog.Error("missing configuration", "name", name)
		os.Exit(2)
	}
	return value
}

func databaseURL() string {
	return strings.TrimSpace(required("PANEL_DATABASE_URL"))
}

func loopback(name string) string {
	addr := required(name)
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		slog.Error("loopback address required", "name", name)
		os.Exit(2)
	}
	return addr
}

func handleSubcommand(args []string, version, commit string) (handled bool, code int) {
	if len(args) == 1 {
		switch args[0] {
		case "serve", "issuer", "migrate":
			return false, 0
		case "--version", "version":
			fmt.Printf("sub2api-panel %s (%s, %s/%s)\n", version, commit, runtime.GOOS, runtime.GOARCH)
			return true, 0
		case "--help", "-h", "help":
			fmt.Print(cliHelp)
			return true, 0
		}
	}
	if len(args) > 0 && args[0] == "update" {
		if err := UpdateCLI(args[1:], version); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true, 1
		}
		return true, 0
	}
	fmt.Fprint(os.Stderr, cliHelp)
	return true, 2
}

func Run(args []string, version, commit string, frontendDist func(string) (fs.FS, error)) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	platform.SetBuildCommit(commit)
	system.SetBuildVersion(version)

	if handled, code := handleSubcommand(args, version, commit); handled {
		if code != 0 {
			os.Exit(code)
		}
		return
	}
	cmd := args[0]
	if cmd == "migrate" {
		if err := platform.MigrateDatabase(context.Background(), databaseURL()); err != nil {
			slog.Error("database migration failed", "error", err)
			os.Exit(2)
		}
		return
	}
	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	secret := required("PANEL_ISSUER_SECRET")
	if len(secret) < 32 {
		slog.Error("invalid issuer secret")
		os.Exit(2)
	}
	db, err := platform.OpenDatabase(context.Background(), databaseURL())
	if err != nil {
		slog.Error("cannot open PostgreSQL database", "error", err)
		os.Exit(2)
	}
	defer db.Close()
	upstreamURL, configured, err := platform.RuntimeUpstream(context.Background(), db, os.Getenv("PANEL_UPSTREAM"))
	if err != nil {
		slog.Error("invalid runtime upstream", "error", err)
		os.Exit(2)
	}
	if cmd == "issuer" && !configured {
		slog.Error("issuer requires a configured upstream")
		os.Exit(2)
	}
	up := platform.NewClient(upstreamURL)
	var address string
	var handler http.Handler
	if cmd == "issuer" {
		address = loopback("PANEL_ISSUER_ADDR")
		handler = (&issuer.Service{
			DB:       db,
			Secret:   secret,
			AdminKey: required("PANEL_UPSTREAM_ADMIN_KEY"),
			Upstream: up,
		}).Routes()
	} else {
		location, err := core.LoadUsageLocation(os.Getenv("PANEL_USAGE_TIMEZONE"))
		if err != nil {
			slog.Error("invalid PANEL_USAGE_TIMEZONE")
			os.Exit(2)
		}
		address = loopback("PANEL_ADDR")
		origin, official := required("PANEL_ORIGIN"), required("PANEL_OFFICIAL_ORIGIN")
		if !platform.CheckOrigin(origin, true) || !platform.CheckOrigin(official, true) || origin == official {
			slog.Error("distinct HTTPS origins are required")
			os.Exit(2)
		}
		dist, err := frontendDist(os.Getenv("PANEL_WEB_DIST"))
		if err != nil {
			slog.Error("frontend assets unavailable", "error", err)
			os.Exit(2)
		}
		issuerClient := platform.NewClient("http://" + loopback("PANEL_ISSUER_ADDR")).WithTimeout(60 * time.Second)
		app := &core.Server{
			Origin:        origin,
			Official:      official,
			Upstream:      up,
			Issuer:        issuerClient,
			PanelUpdater:  panelupdate.NewClient(os.Getenv("PANEL_UPDATE_SOCKET")),
			Secret:        secret,
			Sessions:      platform.NewSessions(),
			UsageLocation: location,
			Database:      db,
			DataDir:       os.Getenv("PANEL_DATA_DIR"),
			Setup:         !configured,
		}
		if raw := os.Getenv("PANEL_OFFICIAL_DATABASE_URL"); raw != "" {
			officialDB, err := observe.OpenOfficialDatabase(context.Background(), raw)
			if err != nil {
				slog.Error("official read-only database unavailable")
				os.Exit(2)
			}
			app.OfficialDB = officialDB
		}
		router.StartRuntime(ctx, app)
		handler = router.NewHandler(app, router.SPAFileServer(dist))
	}
	if os.Getenv("PANEL_LOCAL_READINESS") == "1" {
		handler = withReadiness(handler, db, cmd, version)
	}
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      85 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	slog.Info("panel listening", "address", address, "service", cmd)
	select {
	case err := <-result:
		slog.Error("listener stopped", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 85*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("graceful shutdown timed out", "error", err)
			_ = server.Close()
		}
	}
}
