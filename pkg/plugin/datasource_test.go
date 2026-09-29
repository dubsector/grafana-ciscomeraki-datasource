package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/dubsector/meraki-datasource/pkg/meraki"
)

func testDS(t *testing.T) (*DS, func() string) {
	t.Helper()
	var (
		mu sync.Mutex
		t0 string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		t0 = r.URL.Query().Get("t0")
		mu.Unlock()
		_, _ = w.Write([]byte(`[{"networkId":"N_1","latencyMs":12.5}]`))
	}))
	t.Cleanup(server.Close)
	return &DS{c: meraki.New(server.URL, "org-1", "key")}, func() string {
		mu.Lock()
		defer mu.Unlock()
		return t0
	}
}

func dataQuery(json string, from, to time.Time) backend.DataQuery {
	return backend.DataQuery{RefID: "A", JSON: []byte(json), TimeRange: backend.TimeRange{From: from, To: to}}
}

func TestRunClampsRangeAndAddsNotice(t *testing.T) {
	t.Parallel()
	ds, sentT0 := testDS(t)
	now := time.Now()

	resp := ds.run(context.Background(), dataQuery(`{"queryType":"vpnStats"}`, now.Add(-90*24*time.Hour), now))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}

	t0, err := time.Parse(time.RFC3339, sentT0())
	if err != nil {
		t.Fatalf("t0 sent to Meraki: %v", err)
	}
	if age := now.Sub(t0); age > meraki.VPNStatsWindow.Lookback {
		t.Errorf("t0 is %s old, past the %s lookback", age, meraki.VPNStatsWindow.Lookback)
	}

	meta := resp.Frames[0].Meta
	if meta == nil || len(meta.Notices) != 1 || !strings.Contains(meta.Notices[0].Text, "31 days") {
		t.Errorf("notices = %+v, want one about the 31-day limit", meta)
	}
}

func TestRunLeavesShortRangeAlone(t *testing.T) {
	t.Parallel()
	ds, _ := testDS(t)
	now := time.Now()

	resp := ds.run(context.Background(), dataQuery(`{"queryType":"wirelessLatencyStats","networkId":"N_1"}`, now.Add(-6*time.Hour), now))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if meta := resp.Frames[0].Meta; meta != nil && len(meta.Notices) > 0 {
		t.Errorf("unexpected notices: %+v", meta.Notices)
	}
}

func TestRunRequiresNetworkForWirelessStats(t *testing.T) {
	t.Parallel()
	ds, _ := testDS(t)
	now := time.Now()

	for _, qt := range []string{"wirelessLatencyStats", "wirelessConnectionStats"} {
		resp := ds.run(context.Background(), dataQuery(`{"queryType":"`+qt+`"}`, now.Add(-time.Hour), now))
		if resp.Error == nil || !strings.Contains(resp.Error.Error(), "networkId required") {
			t.Errorf("%s without a network: err = %v", qt, resp.Error)
		}
	}
}
