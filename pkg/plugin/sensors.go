package plugin

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// sensorMetric says where a metric's value sits in a reading and its unit,
// as a Grafana unit ID and as the symbol shown in tables.
type sensorMetric struct {
	path, unit, symbol string
}

// Booleans (door open, water present, ...) become 1 or 0. Metrics missing
// here use their first number or boolean, with no unit.
var sensorMetrics = map[string]sensorMetric{
	"temperature":         {"celsius", "celsius", "°C"},
	"humidity":            {"relativePercentage", "humidity", "%"},
	"battery":             {"percentage", "percent", "%"},
	"co2":                 {"concentration", "ppm", "ppm"},
	"pm25":                {"concentration", "conμgm3", "µg/m³"},
	"tvoc":                {"concentration", "conμgm3", "µg/m³"},
	"indoorAirQuality":    {"score", "none", ""},
	"noise":               {"ambient.level", "dB", "dBA"},
	"apparentPower":       {"draw", "voltamp", "VA"},
	"realPower":           {"draw", "watt", "W"},
	"current":             {"draw", "amp", "A"},
	"voltage":             {"level", "volt", "V"},
	"frequency":           {"level", "hertz", "Hz"},
	"powerFactor":         {"percentage", "percent", "%"},
	"door":                {"open", "", ""},
	"water":               {"present", "bool_yes_no", ""},
	"downstreamPower":     {"enabled", "bool_on_off", ""},
	"remoteLockoutSwitch": {"locked", "bool_yes_no", ""},
}

var fahrenheit = sensorMetric{"fahrenheit", "fahrenheit", "°F"}

func metricInfo(metric string, useFahrenheit bool) sensorMetric {
	if metric == "temperature" && useFahrenheit {
		return fahrenheit
	}
	return sensorMetrics[metric]
}

var sensorColumns = []string{"ts", "name", "serial", "networkName", "metric", "value", "unit", "networkId"}

// sensorRows gives each reading its own flat row with one numeric value.
// Latest rows hold a readings list per sensor; history rows are readings.
// Readings without a number (button presses) are left out.
func sensorRows(rows []json.RawMessage, names map[string]string, useFahrenheit bool) []json.RawMessage {
	type reading struct {
		Ts     string `json:"ts"`
		Metric string `json:"metric"`
	}
	var out []json.RawMessage
	for _, raw := range rows {
		var s struct {
			Serial  string `json:"serial"`
			Network struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"network"`
			Readings []json.RawMessage `json:"readings"`
		}
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		readings := s.Readings
		if readings == nil {
			readings = []json.RawMessage{raw}
		}
		for _, r := range readings {
			var rd reading
			var fields map[string]json.RawMessage
			if json.Unmarshal(r, &rd) != nil || json.Unmarshal(r, &fields) != nil {
				continue
			}
			info := metricInfo(rd.Metric, useFahrenheit)
			v, ok := sensorValue(fields[rd.Metric], info.path)
			if !ok {
				continue
			}
			out = append(out, jsonInOrder(map[string]interface{}{
				"ts": rd.Ts, "name": names[s.Serial], "serial": s.Serial,
				"networkName": s.Network.Name, "networkId": s.Network.ID,
				"metric": rd.Metric, "value": v, "unit": info.symbol,
			}, sensorColumns))
		}
	}
	return out
}

// sensorValue reads the number or boolean at a dotted path, or the first
// one in the object when path is empty.
func sensorValue(raw json.RawMessage, path string) (float64, bool) {
	var leaves []string
	flat := map[string]interface{}{}
	if len(raw) == 0 || flatten(raw, "", &leaves, flat) != nil {
		return 0, false
	}
	if path != "" {
		leaves = []string{path}
	}
	for _, k := range leaves {
		switch v := flat[k].(type) {
		case float64:
			return v, true
		case bool:
			if v {
				return 1, true
			}
			return 0, true
		}
	}
	return 0, false
}

// sensorSeries splits history rows into one time series per sensor and
// metric, so each line is continuous even though sensors report at
// different times.
func sensorSeries(rows []json.RawMessage, useFahrenheit bool) data.Frames {
	type point struct {
		Ts          time.Time `json:"ts"`
		Name        string    `json:"name"`
		Serial      string    `json:"serial"`
		NetworkName string    `json:"networkName"`
		Metric      string    `json:"metric"`
		Value       float64   `json:"value"`
	}
	var order []string
	series := map[string][]point{}
	for _, raw := range rows {
		var p point
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		key := p.Serial + "|" + p.Metric
		if series[key] == nil {
			order = append(order, key)
		}
		series[key] = append(series[key], p)
	}

	frames := data.Frames{}
	for _, key := range order {
		pts := series[key]
		slices.SortStableFunc(pts, func(a, b point) int { return a.Ts.Compare(b.Ts) })
		times := make([]time.Time, len(pts))
		vals := make([]float64, len(pts))
		for i, p := range pts {
			times[i], vals[i] = p.Ts, p.Value
		}
		p := pts[0]
		value := data.NewField("value", data.Labels{
			"name": p.Name, "serial": p.Serial, "network": p.NetworkName, "metric": p.Metric,
		}, vals)
		value.Config = &data.FieldConfig{
			DisplayNameFromDS: strings.TrimSpace(cmp.Or(p.Name, p.Serial) + " " + p.Metric),
			Unit:              metricInfo(p.Metric, useFahrenheit).unit,
		}
		frame := data.NewFrame(p.Metric, data.NewField("ts", nil, times), value)
		frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti, TypeVersion: data.FrameTypeVersion{0, 1}}
		frames = append(frames, frame)
	}
	return frames
}
