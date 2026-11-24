package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/runtimesetting"
)

const runtimeUpstreamKey = "upstream_url"

func runtimeUpstream(ctx context.Context, db *ent.Client, seed string) (string, bool, error) {
	setting, err := db.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(runtimeUpstreamKey)).Only(ctx)
	if err == nil {
		if !checkOrigin(setting.Value, false) {
			return "", false, fmt.Errorf("invalid canonical upstream URL")
		}
		return setting.Value, true, nil
	}
	if !ent.IsNotFound(err) {
		return "", false, err
	}
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return "", false, nil
	}
	if err := validateLocalCandidate(seed); err != nil {
		return "", false, err
	}
	if _, err = db.RuntimeSetting.Create().SetKey(runtimeUpstreamKey).SetValue(seed).SetUpdated(time.Now().Unix()).Save(ctx); err != nil {
		return "", false, err
	}
	return seed, true, nil
}

func setRuntimeUpstream(ctx context.Context, db *ent.Client, raw string) error {
	if err := validateLocalCandidate(raw); err != nil {
		return err
	}
	row, err := db.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(runtimeUpstreamKey)).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = db.RuntimeSetting.Create().SetKey(runtimeUpstreamKey).SetValue(raw).SetUpdated(time.Now().Unix()).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = db.RuntimeSetting.UpdateOne(row).SetValue(raw).SetUpdated(time.Now().Unix()).Save(ctx)
	return err
}

func validateLocalCandidate(raw string) error {
	if !checkOrigin(raw, false) {
		return fmt.Errorf("invalid upstream origin")
	}
	return nil
}

func isLoopbackCandidate(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}

type discoveredSub2API struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

func (p *panel) setupStatus(w http.ResponseWriter, _ *http.Request) error {
	p.runtimeMu.RLock()
	required := p.setup
	p.runtimeMu.RUnlock()
	writeJSON(w, http.StatusOK, struct {
		Required bool `json:"required"`
	}{required})
	return nil
}

func (p *panel) setupDiscover(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Address string `json:"address"`
	}](r)
	if err != nil {
		return err
	}
	result := make([]discoveredSub2API, 0, 2)
	for _, candidate := range localSub2APICandidates(input.Address) {
		found, probeErr := probeSub2API(r.Context(), candidate)
		if probeErr == nil {
			result = append(result, found)
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Candidates []discoveredSub2API `json:"candidates"`
	}{result})
	return nil
}

func (p *panel) setupConfirm(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		URL string `json:"url"`
	}](r)
	if err != nil {
		return err
	}
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	if _, err := probeSub2API(r.Context(), input.URL); err != nil {
		return fail(400, "invalid_upstream")
	}
	if err := setRuntimeUpstream(r.Context(), p.database, input.URL); err != nil {
		return err
	}
	p.runtimeMu.Lock()
	p.upstream = newClient(strings.TrimRight(input.URL, "/"))
	p.setup = false
	p.runtimeMu.Unlock()
	writeJSON(w, http.StatusOK, struct{}{})
	return nil
}

func probeSub2API(ctx context.Context, candidate string) (discoveredSub2API, error) {
	if err := validateLocalCandidate(candidate); err != nil {
		return discoveredSub2API{}, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(candidate, "/")+"/api/v1/settings/public", nil)
	if err != nil {
		return discoveredSub2API{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return discoveredSub2API{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return discoveredSub2API{}, fmt.Errorf("candidate returned status %d", response.StatusCode)
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&envelope); err != nil || envelope.Code != 0 {
		return discoveredSub2API{}, fmt.Errorf("invalid Sub2API public settings response")
	}
	return discoveredSub2API{URL: strings.TrimRight(candidate, "/"), Version: envelope.Data.Version}, nil
}

func localSub2APICandidates(explicit string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, 8)
	add := func(raw string, explicit bool) {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if raw == "" || !checkOrigin(raw, false) {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		if validateLocalCandidate(raw) == nil && (explicit || isLoopbackCandidate(raw)) {
			seen[raw] = struct{}{}
			result = append(result, raw)
		}
	}
	add(explicit, true)
	for _, port := range []int{3000, 8080, 8000} {
		add("http://127.0.0.1:"+strconv.Itoa(port), false)
	}
	return result
}
