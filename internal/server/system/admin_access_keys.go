package system

import (
	"net/http"
	"net/netip"
	"regexp"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const scopedAdminKeysPath = "/admin/settings/admin-api-keys"

var scopedAdminKeyID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

type adminAccessRule struct {
	Permissions      []string `json:"permissions"`
	GroupIDs         []int64  `json:"group_ids"`
	AllGroups        bool     `json:"all_groups"`
	IncludeUngrouped bool     `json:"include_ungrouped"`
	AccountTypes     []string `json:"account_types"`
	Platforms        []string `json:"platforms"`
}

type adminAccessPolicy struct {
	FullAccess        bool              `json:"full_access,omitempty"`
	Rules             []adminAccessRule `json:"rules"`
	GlobalPermissions []string          `json:"global_permissions"`
}

type adminAccessKeyInput struct {
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Enabled           bool              `json:"enabled"`
	Expires           *time.Time        `json:"expires_at"`
	AllowedIPs        []string          `json:"allowed_ips"`
	Policy            adminAccessPolicy `json:"policy"`
	ConfirmFullAccess bool              `json:"confirm_full_access,omitempty"`
}

type scopedAdminKeyView struct {
	ID        string     `json:"id"`
	Prefix    string     `json:"key_prefix"`
	RevokedAt *time.Time `json:"revoked_at"`
	adminAccessKeyInput
}

func validKeyIPRules(rules *[]string) bool {
	if rules == nil {
		return true
	}
	if len(*rules) > 100 {
		return false
	}
	for _, rule := range *rules {
		if len(rule) > 64 {
			return false
		}
		if _, err := netip.ParsePrefix(rule); err != nil {
			if _, addressErr := netip.ParseAddr(rule); addressErr != nil {
				return false
			}
		}
	}
	return true
}

func (h *handler) scopedAdminKeys(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	switch r.Method {
	case "GET":
		result, err := platform.Upstream[struct {
			Items       []scopedAdminKeyView `json:"items"`
			Permissions []struct {
				ID       string `json:"id"`
				Label    string `json:"label"`
				Category string `json:"category"`
				Scope    string `json:"scope"`
			} `json:"permissions"`
		}](r.Context(), h.s.Upstream, "GET", scopedAdminKeysPath, a.Tokens.Access, "", "", nil)
		if result.Items == nil {
			result.Items = []scopedAdminKeyView{}
		}
		return platform.Send(w, 200, result, err)
	default:
		input, err := platform.DecodeBody[struct {
			TOTP   string               `json:"totp"`
			Config *adminAccessKeyInput `json:"config,omitempty"`
		}](r)
		if err != nil {
			return err
		}
		id := r.PathValue("id")
		if id != "" && !scopedAdminKeyID.MatchString(id) {
			return platform.Fail(400, "invalid_input")
		}
		path := scopedAdminKeysPath
		if id != "" {
			path += "/" + id
		}
		action := r.PathValue("action")
		if action != "" {
			if !(r.Method == "POST" && action == "rotate" || r.Method == "DELETE" && action == "record") {
				return platform.Fail(404, "not_found")
			}
			path += "/" + action
		}
		hasConfig := action == "" && r.Method != "DELETE"
		if hasConfig {
			if input.Config == nil {
				return platform.Fail(400, "invalid_input")
			}
			fields := map[string]string{}
			if len(input.Config.Name) < 1 || len(input.Config.Name) > 100 {
				fields["name"] = "名称需在 1 到 100 个字符之间"
			}
			if !validKeyIPRules(&input.Config.AllowedIPs) {
				fields["allowed_ips"] = "请填写合法的 IPv4 / IPv6 地址或 CIDR 网段"
			}
			if input.Config.Policy.FullAccess && !input.Config.ConfirmFullAccess {
				fields["full_access"] = "授予全部管理权限前需明确确认"
			}
			if len(fields) > 0 {
				return platform.FailFields(fields)
			}
		} else if input.Config != nil {
			return platform.Fail(400, "invalid_input")
		}
		if err := h.s.StepUp(r, a, input.TOTP); err != nil {
			return err
		}
		if r.Method == "POST" {
			result, err := platform.Upstream[struct {
				Key  string             `json:"key"`
				Item scopedAdminKeyView `json:"item"`
			}](r.Context(), h.s.Upstream, "POST", path, a.Tokens.Access, "", "", input.Config)
			if err == nil && (result.Key == "" || !scopedAdminKeyID.MatchString(result.Item.ID)) {
				return platform.Fail(502, "invalid_upstream")
			}
			return platform.Send(w, 200, result, err)
		}
		if r.Method == "PUT" {
			result, err := platform.Upstream[scopedAdminKeyView](r.Context(), h.s.Upstream, "PUT", path, a.Tokens.Access, "", "", input.Config)
			if err == nil && result.ID != id {
				return platform.Fail(502, "invalid_upstream")
			}
			return platform.Send(w, 200, result, err)
		}
		_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", path, a.Tokens.Access, "", "", nil)
		return platform.Send(w, 200, struct{}{}, err)
	}
}
