package main

import (
	"net/http"
	"net/url"
	"strings"
)

type adminComplianceView struct {
	Required bool   `json:"required"`
	Version  string `json:"version"`
	Document string `json:"document_url_zh"`
	Phrase   string `json:"ack_phrase_zh"`
}

func (p *panel) adminCompliance(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path := "/admin/compliance"
	var body any
	if r.Method == http.MethodPost {
		input, err := decodeBody[struct {
			Phrase string `json:"phrase"`
		}](r)
		if err != nil {
			return err
		}
		input.Phrase = strings.TrimSpace(input.Phrase)
		if input.Phrase == "" || len(input.Phrase) > 1000 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		path += "/accept"
		body = map[string]string{"phrase": input.Phrase, "language": "zh"}
	}
	status, err := upstream[adminComplianceView](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if status.Required {
		document, err := url.Parse(status.Document)
		if err != nil || document.Scheme != "https" || document.User != nil || document.Host != "github.com" ||
			!strings.HasPrefix(document.Path, "/Wei-Shaw/sub2api/blob/") || status.Version == "" || status.Phrase == "" {
			return fail(http.StatusBadGateway, "invalid_upstream")
		}
	}
	writeJSON(w, http.StatusOK, status)
	return nil
}
