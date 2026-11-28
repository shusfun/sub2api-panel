package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/internal/extplugin"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/pluginrt"
	officialent "sub2api-panel/official"
	"sub2api-panel/official/officialuser"
	"sub2api-panel/plugins/all"
	patrolplugin "sub2api-panel/plugins/patrol"
	"sub2api-panel/plugins/probelmfpd"
)

type Server struct {
	Origin             string
	Official           string
	Upstream           platform.ServiceClient
	Issuer             platform.ServiceClient
	Secret             string
	Sessions           *platform.Sessions
	UsageLocation      *time.Location
	UsageSampleVersion string
	UsageHub           *UsageStreamHub
	OpsHub             *OpsStreamHub
	AccountHub         *OpsStreamHub
	Balances           *BalanceMonitor
	Database           *ent.Client
	OfficialDB         *officialent.Client
	LedgerCards        func(ctx context.Context, ids []int64) (map[int64]LedgerCardSummary, error)
	DataDir            string
	PluginRT           *pluginrt.Runtime
	ExtPlugins         *extplugin.Manager
	DevAccounts        []DevLoginAccount
	Setup              bool
	RuntimeMu          sync.RWMutex
}

type SessionAuth struct {
	User   platform.Identity
	Tokens platform.Credentials
	Local  *platform.Session
}

type Task func(http.ResponseWriter, *http.Request, SessionAuth) error

type DevLoginAccount struct {
	Label    string `json:"label"`
	Email    string `json:"email"`
	Password string `json:"password"`
	OTP      string `json:"otp,omitempty"`
}

func (s *Server) Public(mux *http.ServeMux, pattern string, run func(http.ResponseWriter, *http.Request) error) {
	s.PublicWithLimit(mux, pattern, platform.MaxBody, run)
}

func (s *Server) PublicWithLimit(mux *http.ServeMux, pattern string, limit int64, run func(http.ResponseWriter, *http.Request) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		r, end := platform.TraceRequest(r, w, pattern)
		defer end()
		if r.Method != "GET" && r.Header.Get("Origin") != s.Origin {
			platform.WriteError(w, r, platform.Fail(403, "csrf_rejected"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if limit > platform.MaxBody {
			defer func() { _ = r.Body.Close() }()
		}
		if err := run(w, r); err != nil {
			platform.WriteError(w, r, platform.TranslateUpstream(err))
		}
	})
}

func (s *Server) Route(mux *http.ServeMux, pattern string, admin bool, run Task) {
	s.RouteWithLimit(mux, pattern, admin, platform.MaxBody, run)
}

func (s *Server) RouteWithLimit(mux *http.ServeMux, pattern string, admin bool, limit int64, run Task) {
	s.PublicWithLimit(mux, pattern, limit, func(w http.ResponseWriter, r *http.Request) error {
		s.RuntimeMu.RLock()
		setup := s.Setup
		s.RuntimeMu.RUnlock()
		if setup {
			return platform.Fail(503, "setup_required")
		}
		active, err := s.Sessions.AuthenticateWithLimit(r, s.Origin, time.Now(), limit)
		if err != nil {
			return err
		}
		user, tokens, err := s.Sessions.Principal(r.Context(), s.Upstream, active)
		if err != nil {
			return err
		}
		if admin && user.Role != "admin" {
			return platform.Fail(403, "not_allowed")
		}
		return run(w, r, SessionAuth{User: user, Tokens: tokens, Local: active})
	})
}

func QueryPage[T any, V any](ctx context.Context, c platform.ServiceClient, token, path string, page int, query url.Values, project func(T) (V, error)) (platform.PageResult[V], error) {
	return platform.QueryPage(ctx, c, token, path, page, query, project)
}

func LoadUsageLocation(name string) (*time.Location, error) {
	if name == "" {
		name = "Asia/Shanghai"
	}
	return time.LoadLocation(name)
}

func (s *Server) UsageZone() *time.Location {
	if s.UsageLocation != nil {
		return s.UsageLocation
	}
	location, _ := LoadUsageLocation("")
	return location
}

func UsageScope(userID int64, query string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", userID, query)))
	return hex.EncodeToString(hash[:])
}

func (s *Server) EnsurePlugins() (*pluginrt.Runtime, *extplugin.Manager) {
	s.RuntimeMu.Lock()
	defer s.RuntimeMu.Unlock()
	if s.PluginRT == nil {
		dir := strings.TrimSpace(s.DataDir)
		if dir == "" {
			dir = strings.TrimSpace(os.Getenv("PANEL_DATA_DIR"))
		}
		if dir == "" {
			dir = filepath.Join(os.TempDir(), "sub2api-panel-data")
		}
		s.DataDir = dir
		s.PluginRT = pluginrt.New(dir, all.Manifests(), nil)
	}
	if s.ExtPlugins == nil {
		s.ExtPlugins = extplugin.NewManager(s.DataDir, os.Getenv("PANEL_PLUGIN_MARKET"))
	}
	return s.PluginRT, s.ExtPlugins
}

func (s *Server) PatrolVisibility() (enabled, homeVisible, userVisible, guestEnabled bool, retentionDays int) {
	rt, _ := s.EnsurePlugins()
	on, cfg := rt.ConfigFor(patrolplugin.PatrolPluginID)
	home, _ := cfg["home_visible"].(bool)
	userVis, _ := cfg["user_visible"].(bool)
	ret, _ := cfg["retention_days"].(int)
	if ret < 1 {
		ret = 30
	}
	guestOn, _ := rt.ConfigFor(patrolplugin.GuestPluginID)
	return on, on && home, on && userVis, guestOn, ret
}

func (s *Server) LMFPDService() *probelmfpd.Service {
	rt, _ := s.EnsurePlugins()
	svc, _ := rt.Service(probelmfpd.ServiceName).(*probelmfpd.Service)
	return svc
}

func (s *Server) StepUpEnabled(ctx context.Context, token string) bool {
	settings, err := platform.Upstream[struct {
		StepUp bool `json:"step_up_enabled"`
	}](ctx, s.Upstream, http.MethodGet, "/admin/settings", token, "", "", nil)
	return err == nil && settings.StepUp
}

func (s *Server) RequireOptionalStepUp(r *http.Request, a SessionAuth, code string) error {
	if code != "" || s.StepUpEnabled(r.Context(), a.Tokens.Access) {
		return s.StepUp(r, a, code)
	}
	return nil
}

func (s *Server) StepUp(r *http.Request, a SessionAuth, code string) error {
	ctx, end := platform.StartFlow(r.Context(), "敏感操作二次验证", "actor_id", a.User.ID)
	defer end()
	if !platform.OTPPattern.MatchString(code) {
		return platform.Fail(403, "totp_required")
	}
	result, err := platform.Upstream[struct {
		Verified bool `json:"verified"`
	}](ctx, s.Upstream, "POST", "/user/totp/step-up", a.Tokens.Access, "", "", struct {
		Code string `json:"code"`
	}{code})
	var rejected platform.UpstreamStatus
	if errors.As(err, &rejected) && (rejected.Status == 400 || rejected.Status == 403) || err == nil && !result.Verified {
		return platform.Fail(403, "totp_required")
	}
	return err
}

func (s *Server) ResolveUserLabels(ctx context.Context, token string, ids []int64) map[int64]string {
	ctx, end := platform.StartFlow(ctx, "查询优惠码使用记录", "user_count", len(ids))
	defer end()
	out := make(map[int64]string, len(ids))
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return out
	}
	if s.OfficialDB != nil {
		if rows, err := s.OfficialDB.OfficialUser.Query().Where(officialuser.IDIn(unique...)).All(ctx); err == nil {
			for _, u := range rows {
				label := u.Email
				if label == "" {
					label = u.Username
				}
				if label != "" {
					out[u.ID] = label
				}
			}
		}
	}
	if token == "" || len(unique) > 25 {
		return out
	}
	for _, id := range unique {
		if out[id] != "" {
			continue
		}
		row, err := platform.Upstream[struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		}](ctx, s.Upstream, "GET", fmt.Sprintf("/admin/users/%d", id), token, "", "", nil)
		if err == nil {
			if row.Email != "" {
				out[id] = row.Email
			} else if row.Username != "" {
				out[id] = row.Username
			}
		}
	}
	return out
}
