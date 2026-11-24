package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const previewSampleVersion = "real-models-v1"

type previewModel struct {
	ID     string `json:"id"`
	Name   string `json:"display_name"`
	sample int
}

// Sample numbers keep the mapping stable and give every platform recent usage.
var previewModelCatalog = []struct {
	platform string
	name     string
	models   []previewModel
}{
	{"openai", "OpenAI", []previewModel{
		{"gpt-5.2", "GPT-5.2", 1}, {"gpt-5.4", "GPT-5.4", 3}, {"gpt-5.4-mini", "GPT-5.4 mini", 7},
		{"gpt-5.5", "GPT-5.5", 13}, {"gpt-5.2-pro", "GPT-5.2 pro", 14}, {"gpt-5.5-pro", "GPT-5.5 pro", 18},
	}},
	{"anthropic", "Anthropic", []previewModel{
		{"claude-sonnet-4-6", "Claude Sonnet 4.6", 2}, {"claude-opus-4-6", "Claude Opus 4.6", 8},
		{"claude-haiku-4-5-20251001", "Claude Haiku 4.5", 11}, {"claude-sonnet-4-5-20250929", "Claude Sonnet 4.5", 15},
	}},
	{"gemini", "Gemini", []previewModel{
		{"gemini-2.5-pro", "Gemini 2.5 Pro", 12}, {"gemini-2.5-flash", "Gemini 2.5 Flash", 6},
		{"gemini-3-flash-preview", "Gemini 3 Flash Preview", 16}, {"gemini-3.1-flash-lite", "Gemini 3.1 Flash-Lite", 17},
	}},
	{"grok", "Grok", []previewModel{{"grok-4.3", "Grok 4.3", 4}, {"grok-4.7", "Grok 4.7", 10}}},
	{"deepseek", "DeepSeek", []previewModel{{"deepseek-v4-pro", "DeepSeek V4 Pro", 9}, {"deepseek-flash", "DeepSeek Flash", 5}}},
}

func previewModels(platform string) []previewModel {
	models := []previewModel{}
	for _, provider := range previewModelCatalog {
		if platform == provider.platform || platform == "composite" {
			models = append(models, provider.models...)
		}
	}
	return models
}

func previewModelIDs(platform string) []string {
	models := previewModels(platform)
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func previewCatalogName(platform, kind string, index int) string {
	for _, provider := range previewModelCatalog {
		if platform == provider.platform {
			return fmt.Sprintf("%s %s %02d", provider.name, kind, index+1)
		}
	}
	if platform == "composite" {
		return fmt.Sprintf("复合路由 %02d", index+1)
	}
	return fmt.Sprintf("样本%s %02d", kind, index+1)
}

func (f *fixture) servePreviewModels(w http.ResponseWriter, r *http.Request, path string) bool {
	platform := r.URL.Query().Get("platform")
	if path == "/admin/channels/pricing/sync-models" && r.Method == http.MethodGet {
		officialSuccess(w, map[string]any{"models": previewModelIDs(platform)})
		return true
	}
	parent, remainder, _ := strings.Cut(strings.TrimPrefix(path, "/admin/"), "/")
	idText, endpoint, _ := strings.Cut(remainder, "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return false
	}
	switch parent {
	case "groups":
		index, exists := previewEntryIndex(id, previewGroupBase, f.preview.Groups)
		if endpoint == "model-allowlist-candidates" && r.Method == http.MethodGet {
			if !exists && id != 0 {
				w.WriteHeader(http.StatusNotFound)
				return true
			}
			if exists && platform == "" {
				platform = f.preview.Groups[index].Platform
			}
			officialSuccess(w, map[string]any{"models": previewModelIDs(platform)})
			return true
		}
		if exists && f.preview.Groups[index].Platform == "composite" {
			return servePreviewComposite(w, r, id, endpoint)
		}
	case "accounts":
		if endpoint != "models" && endpoint != "models/sync-upstream" && endpoint != "test" {
			return false
		}
		index, ok := previewEntryIndex(id, previewAccountBase, f.preview.Accounts)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		platform = f.preview.Accounts[index].Platform
		switch {
		case endpoint == "models" && r.Method == http.MethodGet:
			officialSuccess(w, previewModels(platform))
			return true
		case endpoint == "models/sync-upstream" && r.Method == http.MethodPost:
			officialSuccess(w, map[string]any{"models": previewModelIDs(platform)})
			return true
		case endpoint == "test" && r.Method == http.MethodPost:
			return servePreviewAccountTest(w, r, platform)
		}
	}
	return false
}

func servePreviewAccountTest(w http.ResponseWriter, r *http.Request, platform string) bool {
	var input struct {
		Model string `json:"model_id"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	if input.Model == "" {
		if models := previewModels(platform); len(models) > 0 {
			input.Model = models[0].ID
		}
	}
	content, _ := json.Marshal(map[string]string{"type": "content", "text": "模拟上游响应 · " + input.Model})
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(w, "data: %s\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n", content)
	return true
}

func servePreviewComposite(w http.ResponseWriter, r *http.Request, id int64, endpoint string) bool {
	if endpoint != "composite-routes" && endpoint != "composite-routes/preview" {
		return false
	}
	routes := make([]compositeRouteView, 0)
	for _, provider := range previewModelCatalog {
		for _, model := range provider.models {
			routes = append(routes, compositeRouteView{
				ID: int64(model.sample), GroupID: id, PublicModel: model.ID, MatchType: "exact",
				TargetPlatform: provider.platform, UpstreamModel: model.ID, Endpoint: "any", Priority: 10, Enabled: true,
			})
		}
	}
	if endpoint == "composite-routes" && r.Method == http.MethodGet {
		officialSuccess(w, routes)
		return true
	}
	if endpoint != "composite-routes/preview" || r.Method != http.MethodPost {
		return false
	}
	var input struct {
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	for _, route := range routes {
		if route.PublicModel == input.Model {
			officialSuccess(w, map[string]any{
				"matched": true, "group_id": id, "public_model": input.Model, "endpoint": input.Endpoint,
				"source": "route", "target_platform": route.TargetPlatform, "upstream_model": route.UpstreamModel, "route": route,
			})
			return true
		}
	}
	officialSuccess(w, map[string]any{
		"matched": false, "group_id": id, "public_model": input.Model, "endpoint": input.Endpoint, "reason": "模拟目录没有该模型",
	})
	return true
}
