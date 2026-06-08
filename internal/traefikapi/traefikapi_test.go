package traefikapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name":"remind-ui@docker","rule":"Host(` + "`remind.localhost`" + `)","service":"remind-ui","status":"enabled"}]`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	rs, err := c.Routers()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(rs) != 1 || rs[0].Rule != "Host(`remind.localhost`)" {
		t.Fatalf("unexpected routers: %+v", rs)
	}
}
