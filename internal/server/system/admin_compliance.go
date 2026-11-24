package system

import (
	"net/http"
	"net/url"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type adminComplianceView struct {
	Required bool   `json:"required"`
	Version  string `json:"version"`
	Document string `json:"document_url_zh"`
	Phrase   string `json:"ack_phrase_zh"`
}

func (h *handler) adminCompliance(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path := "/admin/compliance"
	var body any
	if r.Method == http.MethodPost {
		input, err := platform.DecodeBody[struct {
			Phrase string `json:"phrase"`
		}](r)
		if err != nil {
			return err
		}
		input.Phrase = strings.TrimSpace(input.Phrase)
		if input.Phrase == "" || len(input.Phrase) > 1000 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		path += "/accept"
		body = map[string]string{"phrase": input.Phrase, "language": "zh"}
	}
	status, err := platform.Upstream[adminComplianceView](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if status.Required {
		document, err := url.Parse(status.Document)
		if err != nil || document.Scheme != "https" || document.User != nil || document.Host != "github.com" ||
			!strings.HasPrefix(document.Path, "/Wei-Shaw/sub2api/blob/") || status.Version == "" || status.Phrase == "" {
			return platform.Fail(http.StatusBadGateway, "invalid_upstream")
		}
	}
	platform.WriteJSON(w, http.StatusOK, status)
	return nil
}
