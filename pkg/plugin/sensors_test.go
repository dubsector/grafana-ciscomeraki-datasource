package plugin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSensorRowsFlattensLatestReadings(t *testing.T) {
	t.Parallel()
	rows := []json.RawMessage{json.RawMessage(`{"serial":"Q3-1","network":{"id":"L_1","name":"Montreal"},"readings":[
		{"ts":"2026-10-05T04:30:00.000000Z","metric":"temperature","temperature":{"fahrenheit":70.02,"celsius":21.12}},
		{"ts":"2026-10-05T04:30:00.000000Z","metric":"noise","noise":{"ambient":{"level":35}}},
		{"ts":"2026-10-05T04:30:00.000000Z","metric":"door","door":{"open":true}},
		{"ts":"2026-10-05T04:30:00.000000Z","metric":"button","button":{"pressType":"short"}},
		{"ts":"2026-10-05T04:30:00.000000Z","metric":"newThing","newThing":{"label":"x","reading":7}}
	]}`)}
	names := map[string]string{"Q3-1": "Freezer"}

	type row struct {
		Name, Serial, NetworkName, NetworkID, Metric, Unit string
		Value                                              float64
	}
	var got []row
	for _, r := range sensorRows(rows, names, false) {
		var x row
		if err := json.Unmarshal(r, &x); err != nil {
			t.Fatal(err)
		}
		got = append(got, x)
	}
	want := []row{
		{"Freezer", "Q3-1", "Montreal", "L_1", "temperature", "°C", 21.12},
		{"Freezer", "Q3-1", "Montreal", "L_1", "noise", "dBA", 35},
		{"Freezer", "Q3-1", "Montreal", "L_1", "door", "", 1},
		{"Freezer", "Q3-1", "Montreal", "L_1", "newThing", "", 7},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	f := sensorRows(rows[:1], names, true)
	var first row
	_ = json.Unmarshal(f[0], &first)
	if first.Value != 70.02 || first.Unit != "°F" {
		t.Errorf("fahrenheit row = %+v, want 70.02 °F", first)
	}
}

func TestSensorSeriesSplitsBySensorAndMetric(t *testing.T) {
	t.Parallel()
	history := []json.RawMessage{
		json.RawMessage(`{"serial":"Q3-1","network":{"id":"L_1","name":"Montreal"},"ts":"2026-10-05T04:15:00Z","metric":"temperature","temperature":{"fahrenheit":68,"celsius":20}}`),
		json.RawMessage(`{"serial":"Q3-2","network":{"id":"L_1","name":"Montreal"},"ts":"2026-10-05T04:15:07Z","metric":"temperature","temperature":{"fahrenheit":50,"celsius":10}}`),
		json.RawMessage(`{"serial":"Q3-1","network":{"id":"L_1","name":"Montreal"},"ts":"2026-10-05T04:00:00Z","metric":"temperature","temperature":{"fahrenheit":69.8,"celsius":21}}`),
		json.RawMessage(`{"serial":"Q3-1","network":{"id":"L_1","name":"Montreal"},"ts":"2026-10-05T04:00:00Z","metric":"humidity","humidity":{"relativePercentage":40}}`),
	}
	frames := sensorSeries(sensorRows(history, map[string]string{"Q3-1": "Freezer"}, false), false)
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3", len(frames))
	}

	f := frames[0]
	if n := f.Rows(); n != 2 {
		t.Fatalf("Freezer temperature has %d points, want 2", n)
	}
	if ts := f.Fields[0].At(0).(time.Time); !ts.Equal(time.Date(2026, 10, 5, 4, 0, 0, 0, time.UTC)) {
		t.Errorf("first point at %s, want the older reading first", ts)
	}
	if v := f.Fields[1].At(0).(float64); v != 21 {
		t.Errorf("first value = %v, want 21", v)
	}
	cfg := f.Fields[1].Config
	if cfg.DisplayNameFromDS != "Freezer temperature" || cfg.Unit != "celsius" {
		t.Errorf("config = %q %q, want Freezer temperature / celsius", cfg.DisplayNameFromDS, cfg.Unit)
	}
	if got := frames[1].Fields[1].Config.DisplayNameFromDS; got != "Q3-2 temperature" {
		t.Errorf("unnamed sensor shows as %q, want its serial", got)
	}
	if got := frames[2].Fields[1].Config.Unit; got != "humidity" {
		t.Errorf("humidity unit = %q", got)
	}
}

func TestSensorSeriesEmpty(t *testing.T) {
	t.Parallel()
	// No frames, so panels say "No data" rather than "no time field".
	if frames := sensorSeries(nil, false); len(frames) != 0 {
		t.Fatalf("got %d frames, want none", len(frames))
	}
}
