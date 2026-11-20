package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dubsector/meraki-datasource/pkg/meraki"
)

func TestResolvedAlertsBecomeRegions(t *testing.T) {
	t.Parallel()
	rows := alertRows([]json.RawMessage{
		json.RawMessage(`{"startedAt":"2026-09-15T10:00:00Z","resolvedAt":"2026-09-15T10:30:00Z","severity":"critical","title":"Unreachable device","categoryType":"connectivity","network":{"name":"Reno"},"scope":{"devices":[{"name":"AP-1"}]}}`),
		json.RawMessage(`{"startedAt":"2026-09-15T11:00:00Z","severity":"warning","title":"CRC errors","network":{"name":"Austin"}}`),
	})
	f := annotationFrame("assuranceAlerts", rows)
	if n, _ := f.RowLen(); n != 2 {
		t.Fatalf("%d annotations, want 2", n)
	}
	byTitle := map[string]int{}
	for i := range 2 {
		byTitle[f.Fields[2].At(i).(string)] = i
	}
	i := byTitle["Unreachable device"]
	if end := f.Fields[1].At(i).(*time.Time); end == nil || !end.Equal(time.Date(2026, 9, 15, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("resolved alert timeEnd = %v", end)
	}
	if got := f.Fields[4].At(i).(string); got != "critical,connectivity" {
		t.Errorf("tags = %q", got)
	}
	if got := f.Fields[3].At(i).(string); got != "Reno · AP-1" {
		t.Errorf("text = %q", got)
	}
	if end := f.Fields[1].At(byTitle["CRC errors"]).(*time.Time); end != nil {
		t.Errorf("open alert has timeEnd %v", end)
	}
}

func TestSecurityEventAnnotation(t *testing.T) {
	t.Parallel()
	f := annotationFrame("securityEvents", []json.RawMessage{json.RawMessage(
		`{"ts":"2026-09-15T10:00:00Z","eventType":"IDS Alert","message":"Port scan","srcIp":"10.0.0.5","destIp":"192.0.2.1","blocked":true,"priority":"1"}`)})
	if got := f.Fields[3].At(0).(string); got != "From 10.0.0.5 to 192.0.2.1, blocked" {
		t.Errorf("text = %q", got)
	}
	if got := f.Fields[4].At(0).(string); got != "IDS Alert,blocked,priority 1" {
		t.Errorf("tags = %q", got)
	}
}

func TestAnnotationsOnlyForEventQueries(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL)
	}))
	t.Cleanup(server.Close)
	ds := &DS{c: meraki.New(server.URL, "org-1", "key")}
	now := time.Now()
	resp := ds.run(context.Background(), dataQuery(`{"queryType":"deviceStatuses","annotation":true}`, now.Add(-time.Hour), now))
	if resp.Error == nil {
		t.Error("device statuses were accepted as annotations")
	}
}
