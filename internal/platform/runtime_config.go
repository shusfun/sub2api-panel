package platform

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

func RuntimeUpstream(ctx context.Context, db *ent.Client, seed string) (string, bool, error) {
	setting, err := db.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(runtimeUpstreamKey)).Only(ctx)
	if err == nil {
		if !CheckOrigin(setting.Value, false) {
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
	if err := ValidateLocalCandidate(seed); err != nil {
		return "", false, err
	}
	if _, err = db.RuntimeSetting.Create().SetKey(runtimeUpstreamKey).SetValue(seed).SetUpdated(time.Now().Unix()).Save(ctx); err != nil {
		return "", false, err
	}
	return seed, true, nil
}

func SetRuntimeUpstream(ctx context.Context, db *ent.Client, raw string) error {
	if err := ValidateLocalCandidate(raw); err != nil {
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

func ValidateLocalCandidate(raw string) error {
	if !CheckOrigin(raw, false) {
		return fmt.Errorf("invalid upstream origin")
	}
	return nil
}

func IsLoopbackCandidate(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}

type DiscoveredSub2API struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

func ProbeSub2API(ctx context.Context, candidate string) (DiscoveredSub2API, error) {
	if err := ValidateLocalCandidate(candidate); err != nil {
		return DiscoveredSub2API{}, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(candidate, "/")+"/api/v1/settings/public", nil)
	if err != nil {
		return DiscoveredSub2API{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return DiscoveredSub2API{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return DiscoveredSub2API{}, fmt.Errorf("candidate returned status %d", response.StatusCode)
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&envelope); err != nil || envelope.Code != 0 {
		return DiscoveredSub2API{}, fmt.Errorf("invalid Sub2API public settings response")
	}
	return DiscoveredSub2API{URL: strings.TrimRight(candidate, "/"), Version: envelope.Data.Version}, nil
}

func LocalSub2APICandidates(explicit string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, 8)
	add := func(raw string, explicit bool) {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if raw == "" || !CheckOrigin(raw, false) {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		if ValidateLocalCandidate(raw) == nil && (explicit || IsLoopbackCandidate(raw)) {
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
