package platform

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const PageSize = 20

type NamedItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type PageResult[T any] struct {
	Items []T  `json:"items"`
	More  bool `json:"more"`
	Pages int  `json:"pages,omitempty"`
}

type WirePage[T any] struct {
	Items []T    `json:"items"`
	Pages *int   `json:"pages"`
	Total *int64 `json:"total"`
}

func WindowPage[T any](items []T) PageResult[T] {
	if len(items) > PageSize {
		return PageResult[T]{Items: items[:PageSize], More: true}
	}
	return PageResult[T]{Items: items}
}

func RequestedPage(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > 10000 {
		return 0, Fail(400, "invalid_input")
	}
	return page, nil
}

func ListQuery(r *http.Request, names ...string) (int, url.Values, error) {
	page, err := RequestedPage(r)
	if err != nil {
		return 0, nil, err
	}
	params := url.Values{}
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if len(value) > 100 {
			return 0, nil, Fail(400, "invalid_input")
		}
		if value != "" {
			params.Set(name, value)
		}
	}
	return page, params, nil
}

func QueryPage[T any, V any](ctx context.Context, c ServiceClient, token, path string, page int, query url.Values, project func(T) (V, error)) (PageResult[V], error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(PageSize))
	input, err := Upstream[WirePage[T]](ctx, c, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return PageResult[V]{}, err
	}
	pages := -1
	if input.Pages != nil {
		pages = *input.Pages
	} else if input.Total != nil && *input.Total >= 0 {
		pages = int((*input.Total + int64(PageSize) - 1) / int64(PageSize))
	}
	if pages < 0 || len(input.Items) > PageSize {
		return PageResult[V]{}, Fail(502, "invalid_upstream")
	}
	items, err := ProjectList(input.Items, project)
	return PageResult[V]{Items: items, More: page < pages, Pages: pages}, err
}

func ProjectList[T any, V any](input []T, project func(T) (V, error)) ([]V, error) {
	out := make([]V, 0, len(input))
	for _, item := range input {
		value, err := project(item)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func PathID(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id < 1 {
		return 0, Fail(400, "invalid_input")
	}
	return id, nil
}

func RequiredID(id int64) error {
	if id < 1 {
		return Fail(502, "invalid_upstream")
	}
	return nil
}

func ParseNonNegativeID(raw string) (int64, error) {
	if raw == "0" {
		return 0, nil
	}
	var id int64
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, Fail(400, "invalid_input")
		}
		id = id*10 + int64(ch-'0')
		if id > 1_000_000_000_000 {
			return 0, Fail(400, "invalid_input")
		}
	}
	if raw == "" {
		return 0, Fail(400, "invalid_input")
	}
	return id, nil
}
