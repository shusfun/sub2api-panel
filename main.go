package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"
)

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		slog.Error("missing configuration", "name", name)
		os.Exit(2)
	}
	return value
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

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if handled, code := runCLI(os.Args[1:]); handled {
		if code != 0 {
			os.Exit(code)
		}
		return
	}
	if os.Args[1] == "migrate" {
		if err := migrateDatabase(context.Background(), databaseURL()); err != nil {
			slog.Error("database migration failed", "error", err)
			os.Exit(2)
		}
		return
	}
	secret := required("PANEL_ISSUER_SECRET")
	if len(secret) < 32 {
		slog.Error("invalid issuer secret")
		os.Exit(2)
	}
	db, err := openDatabase(context.Background(), databaseURL())
	if err != nil {
		slog.Error("cannot open PostgreSQL database", "error", err)
		os.Exit(2)
	}
	defer db.Close()
	upstreamURL, configured, err := runtimeUpstream(context.Background(), db, os.Getenv("PANEL_UPSTREAM"))
	if err != nil {
		slog.Error("invalid runtime upstream", "error", err)
		os.Exit(2)
	}
	if os.Args[1] == "issuer" && !configured {
		slog.Error("issuer requires a configured upstream")
		os.Exit(2)
	}
	up := newClient(upstreamURL)
	var address string
	var handler http.Handler
	if os.Args[1] == "issuer" {
		address = loopback("PANEL_ISSUER_ADDR")
		handler = (&issuer{db: db, secret: secret, adminKey: required("PANEL_UPSTREAM_ADMIN_KEY"), upstream: up}).routes()
	} else {
		location, err := loadUsageLocation(os.Getenv("PANEL_USAGE_TIMEZONE"))
		if err != nil {
			slog.Error("invalid PANEL_USAGE_TIMEZONE")
			os.Exit(2)
		}
		address = loopback("PANEL_ADDR")
		origin, official := required("PANEL_ORIGIN"), required("PANEL_OFFICIAL_ORIGIN")
		if !checkOrigin(origin, true) || !checkOrigin(official, true) || origin == official {
			slog.Error("distinct HTTPS origins are required")
			os.Exit(2)
		}
		dist, err := frontendDist(os.Getenv("PANEL_WEB_DIST"))
		if err != nil {
			slog.Error("frontend assets unavailable", "error", err)
			os.Exit(2)
		}
		issuerClient := newClient("http://" + loopback("PANEL_ISSUER_ADDR"))
		issuerClient.client.Timeout = 60 * time.Second
		app := &panel{origin: origin, official: official, upstream: up, issuer: issuerClient,
			secret: secret, sessions: newSessions(), usageLocation: location, database: db, dataDir: os.Getenv("PANEL_DATA_DIR"), setup: !configured}
		if raw := os.Getenv("PANEL_OFFICIAL_DATABASE_URL"); raw != "" {
			official, err := openOfficialDatabase(context.Background(), raw)
			if err != nil {
				slog.Error("official read-only database unavailable")
				os.Exit(2)
			}
			app.officialDB = official
		}
		app.ensurePlugins()
		app.startPatrolScheduler(context.Background())
		app.startBalanceMonitor(context.Background())
		app.usageHub = newUsageStreamHub(app.buildUsageStreamSnapshot)
		app.usageHub.commit = app.commitUsageSnapshot
		app.opsHub = newOpsStreamHub(app.buildOpsStreamSnapshot)
		app.accountHub = newOpsStreamHub(app.buildAccountActivitySnapshot)
		handler = app.routes(spaFileServer(dist))
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 20 * time.Second, WriteTimeout: 85 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	slog.Info("panel listening", "address", address, "service", os.Args[1])
	if err := server.ListenAndServe(); err != nil {
		slog.Error("listener stopped", "error", err)
		os.Exit(1)
	}
}
