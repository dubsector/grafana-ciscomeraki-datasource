package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestConfigAcceptsStringOrNumberOrgID(t *testing.T) {
	t.Parallel()
	for js, want := range map[string]orgID{
		`{"organizationId":"613605"}`:            "613605",
		`{"organizationId":613605}`:              "613605",
		`{"organizationId":1234567890123456789}`: "1234567890123456789",
	} {
		var cfg config
		if err := json.Unmarshal([]byte(js), &cfg); err != nil {
			t.Errorf("%s: %v", js, err)
		} else if cfg.OrganizationID != want {
			t.Errorf("%s: got %q, want %q", js, cfg.OrganizationID, want)
		}
		if _, err := NewDS(context.Background(), backend.DataSourceInstanceSettings{JSONData: []byte(js)}); err != nil {
			t.Errorf("NewDS(%s): %v", js, err)
		}
	}

	var cfg config
	if err := json.Unmarshal([]byte(`{"organizationId":true}`), &cfg); err == nil {
		t.Error("a boolean organizationId was accepted")
	}
}

func TestBuildFrameKeepsAPIFieldOrder(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{
		json.RawMessage(`{"serial":"Q2-1","status":"online","network":{"id":"N_1","name":"HQ"},"mac":"aa"}`),
		json.RawMessage(`{"serial":"Q2-2","status":"offline","network":{"id":"N_2","name":"Lab"},"mac":"bb","tags":["x"]}`),
	}
	want := []string{"serial", "status", "network.id", "network.name", "mac", "tags"}

	// Map iteration is random, so one lucky pass proves nothing.
	for range 20 {
		frame, err := buildFrame("t", rows)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range frame.Fields {
			got = append(got, f.Name)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("fields = %v, want %v", got, want)
		}
	}
}

func TestExplodeGivesEachUplinkARow(t *testing.T) {
	t.Parallel()
	rows := explode([]json.RawMessage{
		json.RawMessage(`{"serial":"Q2-1","uplinks":[{"interface":"wan1","status":"active"},{"interface":"wan2","status":"ready"}],"model":"MX75"}`),
		json.RawMessage(`{"serial":"Q2-2","uplinks":[],"model":"MX68"}`),
	}, "uplinks")

	want := []string{
		`{"serial":"Q2-1","interface":"wan1","status":"active","model":"MX75"}`,
		`{"serial":"Q2-1","interface":"wan2","status":"ready","model":"MX75"}`,
		`{"serial":"Q2-2","uplinks":[],"model":"MX68"}`,
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %s", len(rows), len(want), rows)
	}
	for i := range want {
		if string(rows[i]) != want[i] {
			t.Errorf("row %d = %s, want %s", i, rows[i], want[i])
		}
	}
}

func TestVPNPeerRowsJoinSummariesPerUplinkPair(t *testing.T) {
	t.Parallel()
	rows := vpnPeerRows([]json.RawMessage{
		json.RawMessage(`{"networkId":"N_1","networkName":"London","merakiVpnPeers":[{"networkId":"N_2","networkName":"HQ",
			"usageSummary":{"receivedInKilobytes":20,"sentInKilobytes":10},
			"latencySummaries":[{"senderUplink":"wan1","receiverUplink":"wan1","avgLatencyMs":150},{"senderUplink":"wan2","receiverUplink":"wan1","avgLatencyMs":90}],
			"lossPercentageSummaries":[{"senderUplink":"wan1","receiverUplink":"wan1","avgLossPercentage":0.5}]}]}`),
		json.RawMessage(`{"networkId":"N_3","networkName":"Lonely","merakiVpnPeers":[]}`),
	})

	want := []string{
		`{"networkName":"London","peerNetworkName":"HQ","senderUplink":"wan1","receiverUplink":"wan1","sentInKilobytes":10,"receivedInKilobytes":20,"avgLatencyMs":150,"avgLossPercentage":0.5,"networkId":"N_1","peerNetworkId":"N_2"}`,
		`{"networkName":"London","peerNetworkName":"HQ","senderUplink":"wan2","receiverUplink":"wan1","sentInKilobytes":10,"receivedInKilobytes":20,"avgLatencyMs":90,"networkId":"N_1","peerNetworkId":"N_2"}`,
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %s", len(rows), len(want), rows)
	}
	for i := range want {
		if string(rows[i]) != want[i] {
			t.Errorf("row %d =\n%s\nwant\n%s", i, rows[i], want[i])
		}
	}
}

func TestWithNetworkNames(t *testing.T) {
	t.Parallel()
	rows := withNetworkNames([]json.RawMessage{
		json.RawMessage(`{"serial":"Q2-1","network":{"id":"N_1"},"status":"online"}`),
		json.RawMessage(`{"serial":"Q2-2","network":{"id":"N_9"},"status":"offline"}`),
	}, map[string]string{"N_1": "HQ"})

	want := []string{
		`{"serial":"Q2-1","network":{"id":"N_1","name":"HQ"},"status":"online"}`,
		`{"serial":"Q2-2","network":{"id":"N_9"},"status":"offline"}`,
	}
	for i := range want {
		if string(rows[i]) != want[i] {
			t.Errorf("row %d = %s, want %s", i, rows[i], want[i])
		}
	}
}

func TestBuildFrameShowsListsAsText(t *testing.T) {
	t.Parallel()
	frame, err := buildFrame("t", []json.RawMessage{
		json.RawMessage(`{"errors":["CRC errors","Port flapping"],"nested":[{"a":1}]}`),
		json.RawMessage(`{"errors":[],"nested":[]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"errors": {"CRC errors, Port flapping", ""},
		"nested": {`[{"a":1}]`, ""},
	}
	for _, f := range frame.Fields {
		for i, w := range want[f.Name] {
			if got, _ := f.ConcreteAt(i); got != w {
				t.Errorf("%s[%d] = %q, want %q", f.Name, i, got, w)
			}
		}
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
