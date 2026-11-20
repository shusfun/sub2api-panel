package main

import (
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
	if len(os.Args) != 2 || os.Args[1] != "serve" && os.Args[1] != "issuer" {
		slog.Error("usage: panel serve | panel issuer")
		os.Exit(2)
	}
	upstreamURL := required("PANEL_UPSTREAM")
	secret := required("PANEL_ISSUER_SECRET")
	if !checkOrigin(upstreamURL, false) || len(secret) < 32 {
		slog.Error("invalid upstream origin or issuer secret")
		os.Exit(2)
	}
	up := newClient(upstreamURL)
	var address string
	var handler http.Handler
	if os.Args[1] == "issuer" {
		address = loopback("PANEL_ISSUER_ADDR")
		db, err := openIssuerDB(required("PANEL_ISSUER_DB"))
		if err != nil {
			slog.Error("cannot open issuer database")
			os.Exit(2)
		}
		defer db.Close()
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
		dist := os.DirFS(required("PANEL_WEB_DIST"))
		issuerClient := newClient("http://" + loopback("PANEL_ISSUER_ADDR"))
		issuerClient.client.Timeout = 60 * time.Second
		app := &panel{origin: origin, official: official, upstream: up, issuer: issuerClient,
			secret: secret, sessions: newSessions(), usageLocation: location}
		handler = app.routes(http.FileServerFS(dist))
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 20 * time.Second, WriteTimeout: 85 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	slog.Info("panel listening", "address", address, "service", os.Args[1])
	if err := server.ListenAndServe(); err != nil {
		slog.Error("listener stopped", "error", err)
		os.Exit(1)
	}
}
