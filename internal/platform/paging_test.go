package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func pageServer(t *testing.T, data string) ServiceClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":` + data + `}`))
	}))
	t.Cleanup(srv.Close)
	return ServiceClient{Base: srv.URL, Client: srv.Client()}
}

func identity(v int) (int, error) { return v, nil }

func TestQueryPageReturnsUpstreamTotal(t *testing.T) {
	c := pageServer(t, `{"items":[1,2],"total":45}`)
	got, err := QueryPage[int, int](context.Background(), c, "token", "/admin/x", 1, nil, identity)
	if err != nil || got.Total == nil || *got.Total != 45 || got.Pages != 3 || !got.More {
		t.Fatalf("unexpected page: %+v err=%v", got, err)
	}
}

func TestQueryPageOmitsTotalWhenUpstreamOmitsIt(t *testing.T) {
	c := pageServer(t, `{"items":[1],"pages":1}`)
	got, err := QueryPage[int, int](context.Background(), c, "token", "/admin/x", 1, nil, identity)
	if err != nil || got.Total != nil || got.Pages != 1 || got.More {
		t.Fatalf("unexpected page: %+v err=%v", got, err)
	}
}

func TestQueryPageRejectsNegativeTotal(t *testing.T) {
	for _, data := range []string{`{"items":[],"total":-1}`, `{"items":[],"pages":1,"total":-1}`} {
		c := pageServer(t, data)
		if _, err := QueryPage[int, int](context.Background(), c, "token", "/admin/x", 1, nil, identity); err == nil {
			t.Fatalf("negative total accepted: %s", data)
		}
	}
}
