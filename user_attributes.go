package main

import (
	"fmt"
	"net/http"
)

type userAttributeDefinition struct {
	namedItem
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Options     []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"options"`
}

type userAttributeValue struct {
	ID    int64  `json:"attribute_id"`
	Value string `json:"value"`
}

func (p *panel) userAttributeDefinitions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	result, err := upstream[[]userAttributeDefinition](r.Context(), p.upstream, http.MethodGet, "/admin/user-attributes?enabled=true", a.tokens.Access, "", "", nil)
	if result == nil {
		result = []userAttributeDefinition{}
	}
	return send(w, http.StatusOK, result, err)
}

func (p *panel) userAttributes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	var payload any
	if r.Method == http.MethodPut {
		input, err := decodeBody[struct {
			Values map[int64]string `json:"values"`
		}](r)
		if err != nil || len(input.Values) == 0 || len(input.Values) > 100 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		for id, value := range input.Values {
			if id < 1 || len(value) > settingTextLimit {
				return fail(http.StatusBadRequest, "invalid_input")
			}
		}
		payload = input
	}
	result, err := upstream[[]userAttributeValue](r.Context(), p.upstream, r.Method, fmt.Sprintf("/admin/users/%d/attributes", id), a.tokens.Access, "", "", payload)
	if result == nil {
		result = []userAttributeValue{}
	}
	return send(w, http.StatusOK, result, err)
}
