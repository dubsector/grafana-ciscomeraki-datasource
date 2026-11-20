package plugin

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// defaultProbeIP is the target Meraki appliances probe unless configured otherwise.
const defaultProbeIP = "8.8.8.8"

// uplinkSummaryRows turns the org's five-minute loss and latency samples into
// one row per uplink and probe target, worst first.
func uplinkSummaryRows(rows []json.RawMessage, networkID string, networks, devices map[string]string) []json.RawMessage {
	type sample struct {
		Ts          time.Time `json:"ts"`
		LossPercent *float64  `json:"lossPercent"`
		LatencyMs   *float64  `json:"latencyMs"`
	}
	type uplink struct {
		NetworkID  string   `json:"networkId"`
		Serial     string   `json:"serial"`
		Uplink     string   `json:"uplink"`
		IP         string   `json:"ip"`
		TimeSeries []sample `json:"timeSeries"`
	}
	type row struct {
		NetworkName   string   `json:"networkName"`
		Name          string   `json:"name"`
		Serial        string   `json:"serial"`
		Uplink        string   `json:"uplink"`
		IP            string   `json:"ip"`
		LossPercent   *float64 `json:"lossPercent"`
		LatencyMs     *float64 `json:"latencyMs"`
		MaxLatencyMs  *float64 `json:"maxLatencyMs"`
		LatestLoss    *float64 `json:"latestLossPercent"`
		LatestLatency *float64 `json:"latestLatencyMs"`
	}
	out := make([]row, 0, len(rows))
	for _, raw := range rows {
		var u uplink
		if json.Unmarshal(raw, &u) != nil || (networkID != "" && u.NetworkID != networkID) {
			continue
		}
		r := row{NetworkName: networks[u.NetworkID], Name: devices[u.Serial], Serial: u.Serial, Uplink: u.Uplink, IP: u.IP}
		var loss, lat []float64
		for _, s := range u.TimeSeries {
			if s.LossPercent != nil {
				loss = append(loss, *s.LossPercent)
				r.LatestLoss = s.LossPercent
			}
			if s.LatencyMs != nil {
				lat = append(lat, *s.LatencyMs)
				r.LatestLatency = s.LatencyMs
			}
		}
		r.LossPercent, r.LatencyMs = mean(loss), mean(lat)
		if len(lat) > 0 {
			m := slices.Max(lat)
			r.MaxLatencyMs = &m
		}
		out = append(out, r)
	}
	val := func(p *float64) float64 {
		if p == nil {
			return -1
		}
		return *p
	}
	slices.SortStableFunc(out, func(a, b row) int {
		return cmp.Or(cmp.Compare(val(b.LossPercent), val(a.LossPercent)), cmp.Compare(val(b.LatencyMs), val(a.LatencyMs)))
	})
	return marshalRows(out)
}

// lossLatencySeries is one uplink's loss, latency and jitter as a time series.
func lossLatencySeries(rows []json.RawMessage, serial, uplink string) data.Frames {
	type point struct {
		StartTime   time.Time `json:"startTime"`
		LossPercent *float64  `json:"lossPercent"`
		LatencyMs   *float64  `json:"latencyMs"`
		Jitter      *float64  `json:"jitter"`
	}
	var times []time.Time
	var loss, lat, jitter []*float64
	for _, raw := range rows {
		var p point
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		times = append(times, p.StartTime)
		loss, lat, jitter = append(loss, p.LossPercent), append(lat, p.LatencyMs), append(jitter, p.Jitter)
	}
	labels := data.Labels{"serial": serial, "uplink": uplink}
	frame := data.NewFrame("uplinkLossLatencyHistory",
		data.NewField("ts", nil, times),
		withUnit(data.NewField("loss", labels, loss), "percent", "Loss"),
		withUnit(data.NewField("latency", labels, lat), "ms", "Latency"),
		withUnit(data.NewField("jitter", labels, jitter), "ms", "Jitter"),
	)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesWide, TypeVersion: data.FrameTypeVersion{0, 1}}
	return data.Frames{frame}
}

// uplinkUsageSeries turns bytes per interval into bits per second sent and
// received on each WAN uplink.
func uplinkUsageSeries(rows []json.RawMessage) data.Frames {
	type iface struct {
		Interface string  `json:"interface"`
		Sent      float64 `json:"sent"`
		Received  float64 `json:"received"`
	}
	type bucket struct {
		StartTime   time.Time `json:"startTime"`
		EndTime     time.Time `json:"endTime"`
		ByInterface []iface   `json:"byInterface"`
	}
	var buckets []bucket
	var names []string
	for _, raw := range rows {
		var b bucket
		if json.Unmarshal(raw, &b) != nil {
			continue
		}
		for _, i := range b.ByInterface {
			if !slices.Contains(names, i.Interface) {
				names = append(names, i.Interface)
			}
		}
		buckets = append(buckets, b)
	}
	times := make([]time.Time, len(buckets))
	sent := map[string][]*float64{}
	recv := map[string][]*float64{}
	for i, b := range buckets {
		times[i] = b.StartTime
		secs := b.EndTime.Sub(b.StartTime).Seconds()
		for _, n := range names {
			sent[n] = append(sent[n], nil)
			recv[n] = append(recv[n], nil)
		}
		if secs <= 0 {
			continue
		}
		for _, f := range b.ByInterface {
			s, r := f.Sent*8/secs, f.Received*8/secs
			sent[f.Interface][i], recv[f.Interface][i] = &s, &r
		}
	}
	// NewField copies the slice, so the time field is built after it is filled.
	fields := []*data.Field{data.NewField("ts", nil, times)}
	for _, n := range names {
		l := data.Labels{"uplink": n}
		fields = append(fields,
			withUnit(data.NewField("sent", l, sent[n]), "bps", n+" sent"),
			withUnit(data.NewField("received", l, recv[n]), "bps", n+" received"))
	}
	frame := data.NewFrame("applianceUplinkUsage", fields...)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesWide, TypeVersion: data.FrameTypeVersion{0, 1}}
	return data.Frames{frame}
}

// alertRows keeps the columns an operator scans: when, how bad, what, where.
func alertRows(rows []json.RawMessage) []json.RawMessage {
	type device struct {
		Name   string `json:"name"`
		Serial string `json:"serial"`
	}
	type alert struct {
		StartedAt    time.Time  `json:"startedAt"`
		ResolvedAt   *time.Time `json:"resolvedAt"`
		Severity     string     `json:"severity"`
		Title        string     `json:"title"`
		Description  string     `json:"description"`
		CategoryType string     `json:"categoryType"`
		Type         string     `json:"type"`
		DeviceType   string     `json:"deviceType"`
		Network      struct {
			Name string `json:"name"`
		} `json:"network"`
		Scope struct {
			Devices []device `json:"devices"`
		} `json:"scope"`
	}
	type row struct {
		StartedAt   time.Time  `json:"startedAt"`
		ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
		Severity    string     `json:"severity"`
		Title       string     `json:"title"`
		NetworkName string     `json:"networkName"`
		Devices     string     `json:"devices"`
		Description string     `json:"description"`
		Category    string     `json:"category"`
		Type        string     `json:"type"`
		DeviceType  string     `json:"deviceType"`
	}
	out := make([]row, 0, len(rows))
	for _, raw := range rows {
		var a alert
		if json.Unmarshal(raw, &a) != nil {
			continue
		}
		names := make([]string, 0, len(a.Scope.Devices))
		for _, d := range a.Scope.Devices {
			names = append(names, cmp.Or(d.Name, d.Serial))
		}
		out = append(out, row{a.StartedAt, a.ResolvedAt, a.Severity, a.Title, a.Network.Name, strings.Join(names, ", "),
			a.Description, a.CategoryType, a.Type, a.DeviceType})
	}
	rank := map[string]int{"critical": 0, "warning": 1, "informational": 2}
	sev := func(s string) int {
		if r, ok := rank[s]; ok {
			return r
		}
		return len(rank)
	}
	slices.SortStableFunc(out, func(a, b row) int {
		return cmp.Or(cmp.Compare(sev(a.Severity), sev(b.Severity)), b.StartedAt.Compare(a.StartedAt))
	})
	return marshalRows(out)
}

func withUnit(f *data.Field, unit, name string) *data.Field {
	f.Config = &data.FieldConfig{Unit: unit, DisplayNameFromDS: name}
	return f
}

func mean(v []float64) *float64 {
	if len(v) == 0 {
		return nil
	}
	var s float64
	for _, x := range v {
		s += x
	}
	m := s / float64(len(v))
	return &m
}

func marshalRows[T any](rows []T) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(rows))
	for _, r := range rows {
		b, _ := json.Marshal(r)
		out = append(out, b)
	}
	return out
}

func uplinkOrDefault(u string) string {
	return cmp.Or(u, "wan1")
}
