package meraki

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPanelsAskingTheSameThingShareOneRequest(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`[{"id":"N_1"}]`))
	}))
	defer server.Close()

	c := New(server.URL, "org-1", "key")
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			rows, err := c.Networks(context.Background())
			if err != nil || len(rows) != 1 {
				t.Errorf("rows %s, err %v", rows, err)
			}
		})
	}
	wg.Wait()
	if _, err := c.Networks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
	if _, err := c.ForOrg("org-2").Networks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 2 {
		t.Error("another organization's networks came from the cache")
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, `{"errors":["nope"]}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := New(server.URL, "org-1", "key")
	if _, err := c.Networks(context.Background()); err == nil {
		t.Fatal("first call should fail")
	}
	if _, err := c.Networks(context.Background()); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

func TestLimiterAllowsABurstThenPaces(t *testing.T) {
	t.Parallel()
	l := &limiter{interval: 20 * time.Millisecond, burst: 3}
	start := time.Now()
	for range 3 {
		if err := l.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d > 10*time.Millisecond {
		t.Errorf("burst took %v", d)
	}
	for range 3 {
		_ = l.wait(context.Background())
	}
	if d := time.Since(start); d < 55*time.Millisecond {
		t.Errorf("6 requests took %v, want about 60ms", d)
	}
}

func TestLimiterStopsWaitingWhenCancelled(t *testing.T) {
	t.Parallel()
	l := &limiter{interval: time.Hour, burst: 1}
	_ = l.wait(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := l.wait(ctx); err == nil {
		t.Error("wait ignored the cancelled context")
	}
}
