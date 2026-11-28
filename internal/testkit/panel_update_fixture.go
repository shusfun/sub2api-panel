package testkit

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"sub2api-panel/internal/panelupdate"
	"sub2api-panel/internal/platform"
)

func setupUpdateFixture(t testing.TB, f *fixture) {
	t.Helper()
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "update.sock"))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var job *panelupdate.Job
	offer := panelupdate.ReleaseInfo{Version: "0.1.0001", Notes: "演练发布说明", URL: "https://github.com/shusfun/sub2api-panel/releases/tag/v0.1.1", Availability: "ready", Migration: true}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if job != nil && time.Since(job.UpdatedAt) > 3*time.Second && job.Phase != "succeeded" {
			job = &panelupdate.Job{ID: job.ID, From: job.From, To: job.To, Phase: "succeeded", UpdatedAt: time.Now().UTC()}
		}
		platform.WriteJSON(w, 200, panelupdate.Status{Configured: true, Job: job})
	})
	mux.HandleFunc("GET /release", func(w http.ResponseWriter, r *http.Request) {
		platform.WriteJSON(w, 200, panelupdate.Check{Release: &offer})
	})
	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var input panelupdate.Request
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Actor != 1 || input.Version != offer.Version {
			w.WriteHeader(400)
			return
		}
		if !platform.SpanHex.MatchString(r.Header.Get("traceparent")) {
			t.Error("update request is missing the panel trace")
		}
		if job != nil {
			w.WriteHeader(409)
			return
		}
		job = &panelupdate.Job{ID: platform.RandomHex(16), From: "0.1.0-rc.3", To: input.Version, Phase: "queued", UpdatedAt: time.Now().UTC()}
		platform.WriteJSON(w, 202, job)
	})
	server := &http.Server{Handler: mux}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.Serve(listener) }()
	f.panel.PanelUpdater = panelupdate.NewClient(listener.Addr().String())
}
