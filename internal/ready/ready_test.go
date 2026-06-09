package ready

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitReady_BecomesReady(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 2 {
			w.WriteHeader(http.StatusBadGateway) // 502 first
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := WaitReady([]string{srv.URL}, 3*time.Second, srv.Client()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
}

func TestWaitReady_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if err := WaitReady([]string{srv.URL}, 600*time.Millisecond, srv.Client()); err == nil {
		t.Fatal("expected timeout error")
	}
}
