package plugin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUplinkSummaryRowsWorstFirst(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{
		json.RawMessage(`{"networkId":"N_1","serial":"Q1","uplink":"wan1","ip":"8.8.8.8","timeSeries":[{"ts":"2026-09-15T12:00:00Z","lossPercent":0,"latencyMs":10},{"ts":"2026-09-15T12:01:00Z","lossPercent":0,"latencyMs":20}]}`),
		json.RawMessage(`{"networkId":"N_2","serial":"Q2","uplink":"wan2","ip":"8.8.8.8","timeSeries":[{"ts":"2026-09-15T12:00:00Z","lossPercent":4,"latencyMs":30},{"ts":"2026-09-15T12:01:00Z","lossPercent":2,"latencyMs":50}]}`),
	}
	got := uplinkSummaryRows(rows, "", map[string]string{"N_2": "Austin"}, map[string]string{"Q2": "MX-AUS"})
	var first struct {
		NetworkName   string
		Name          string
		LossPercent   float64
		LatencyMs     float64
		MaxLatencyMs  float64
		LatestLatency float64 `json:"latestLatencyMs"`
	}
	if err := json.Unmarshal(got[0], &first); err != nil {
		t.Fatal(err)
	}
	if first.Name != "MX-AUS" || first.NetworkName != "Austin" || first.LossPercent != 3 || first.LatencyMs != 40 || first.MaxLatencyMs != 50 || first.LatestLatency != 50 {
		t.Errorf("worst uplink row = %+v", first)
	}
	if only := uplinkSummaryRows(rows, "N_1", nil, nil); len(only) != 1 {
		t.Errorf("network filter kept %d rows, want 1", len(only))
	}
}

func TestUplinkUsageSeriesBitsPerSecond(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{json.RawMessage(`{"startTime":"2026-09-15T12:00:00Z","endTime":"2026-09-15T12:01:00Z","byInterface":[{"interface":"wan1","sent":600,"received":1200}]}`)}
	f := uplinkUsageSeries(rows)[0]
	if len(f.Fields) != 3 {
		t.Fatalf("got %d fields, want ts, sent, received", len(f.Fields))
	}
	sent, _ := f.Fields[1].ConcreteAt(0)
	recv, _ := f.Fields[2].ConcreteAt(0)
	if sent.(float64) != 80 || recv.(float64) != 160 {
		t.Errorf("sent %v, received %v bps; want 80 and 160", sent, recv)
	}
	if ts, _ := f.Fields[0].ConcreteAt(0); !ts.(time.Time).Equal(time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("time %v, want the bucket start", ts)
	}
}

func TestAlertRowsMostSevereFirst(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{
		json.RawMessage(`{"startedAt":"2026-09-15T10:00:00Z","severity":"warning","title":"CRC errors","network":{"name":"Austin"},"scope":{"devices":[{"name":"SW-1"}]}}`),
		json.RawMessage(`{"startedAt":"2026-09-14T10:00:00Z","severity":"critical","title":"Unreachable","network":{"name":"Reno"},"scope":{"devices":[{"serial":"Q2"},{"name":"CAM-2"}]}}`),
	}
	got := alertRows(rows)
	var first struct{ Severity, Title, NetworkName, Devices string }
	if err := json.Unmarshal(got[0], &first); err != nil {
		t.Fatal(err)
	}
	if first.Severity != "critical" || first.Devices != "Q2, CAM-2" || first.NetworkName != "Reno" {
		t.Errorf("first alert = %+v", first)
	}
}

func TestTopClientRowsFilterByNetwork(t *testing.T) {
	t.Parallel()
	rows := topClientRows([]json.RawMessage{
		json.RawMessage(`{"name":"NAS","mac":"00:11","network":{"id":"N_1","name":"HQ"},"usage":{"total":10,"upstream":9,"downstream":1,"percentage":50}}`),
		json.RawMessage(`{"name":"Phone","mac":"00:22","network":{"id":"N_2","name":"Austin"},"usage":{"total":5}}`),
	}, "N_1")
	if len(rows) != 1 || string(rows[0]) != `{"name":"NAS","mac":"00:11","networkName":"HQ","total":10,"downstream":1,"upstream":9,"percentage":50}` {
		t.Errorf("rows = %s", rows)
	}
}
