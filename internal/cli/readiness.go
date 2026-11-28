package cli

import (
	"context"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/internal/platform"
)

func withReadiness(next http.Handler, db *ent.Client, service, version string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", next)
	mux.HandleFunc("GET /internal/panel-ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer cancel()
		if err := readyDatabase(ctx, db, service); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		platform.WriteJSON(w, http.StatusOK, struct {
			Version string `json:"version"`
			Service string `json:"service"`
		}{version, service})
	})
	return mux
}

func readyDatabase(ctx context.Context, db *ent.Client, service string) error {
	if _, err := db.RuntimeSetting.Query().Exist(ctx); err != nil {
		return err
	}
	if service == "issuer" {
		for _, probe := range []func(context.Context) (bool, error){
			db.Grant.Query().Exist, db.Issue.Query().Exist, db.GrantAudit.Query().Exist,
		} {
			if _, err := probe(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	for _, probe := range []func(context.Context) (bool, error){
		db.UsageCursor.Query().Exist, db.UsageObservation.Query().Exist,
		db.UsageSnapshot.Query().Exist, db.UsageVersion.Query().Exist,
		db.PatrolPolicy.Query().Exist, db.PatrolRun.Query().Exist, db.ProbeTemplate.Query().Exist,
		db.LedgerAccount.Query().Exist, db.LedgerAlert.Query().Exist, db.LedgerDay.Query().Exist,
		db.LedgerEvent.Query().Exist, db.LedgerHour.Query().Exist, db.LedgerSample.Query().Exist, db.LedgerWallet.Query().Exist,
	} {
		if _, err := probe(ctx); err != nil {
			return err
		}
	}
	return nil
}
