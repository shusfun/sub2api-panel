package main

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var customSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)

type customEndpoint struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description,omitempty"`
}

type customMenuItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	URL        string `json:"url"`
	Slug       string `json:"page_slug"`
	Icon       string `json:"icon_svg,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Sort       int    `json:"sort_order"`
	HideOpen   bool   `json:"hide_open_button,omitempty"`
}

func externalLink(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return nil, false
	}
	return u, true
}

func projectCustomEndpoints(items []customEndpoint) []customEndpoint {
	out := make([]customEndpoint, 0)
	for _, item := range items {
		if _, ok := externalLink(item.Endpoint); ok && item.Name != "" {
			out = append(out, item)
		}
	}
	return out
}

func projectCustomMenu(items []customMenuItem, visibility string) []customMenuItem {
	out := make([]customMenuItem, 0)
	seen := make(map[string]bool)
	for _, item := range items {
		if item.Visibility != visibility || strings.TrimSpace(item.Label) == "" {
			continue
		}
		if item.Slug == "" {
			item.Slug = item.ID
		}
		if _, ok := externalLink(item.URL); !ok || !customSlugPattern.MatchString(item.Slug) || seen[item.Slug] {
			continue
		}
		item.Visibility = ""
		if len(item.Icon) > 8192 || !strings.HasPrefix(strings.TrimSpace(item.Icon), "<svg") {
			item.Icon = ""
		}
		seen[item.Slug] = true
		out = append(out, item)
	}
	slices.SortStableFunc(out, func(a, b customMenuItem) int {
		if a.Sort < b.Sort {
			return -1
		}
		if a.Sort > b.Sort {
			return 1
		}
		return 0
	})
	return out
}

func (p *panel) adminMenu(r *http.Request, a sessionAuth) ([]customMenuItem, error) {
	result, err := upstream[struct {
		Items []customMenuItem `json:"custom_menu_items"`
	}](r.Context(), p.upstream, "GET", "/admin/settings", a.tokens.Access, "", "", nil)
	if err != nil {
		return nil, err
	}
	return projectCustomMenu(result.Items, "admin"), nil
}

func (p *panel) adminCustomMenu(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	items, err := p.adminMenu(r, a)
	return send(w, 200, items, err)
}
