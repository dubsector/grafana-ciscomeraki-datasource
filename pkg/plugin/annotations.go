package plugin

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// annotationTypes are the query types that make sense as events on a chart.
var annotationTypes = map[string]bool{"networkEvents": true, "securityEvents": true, "assuranceAlerts": true}

type annotation struct {
	time, end   time.Time
	title, text string
	tags        []string
}

// annotationFrame gives Grafana the fields it maps to annotations by name:
// time, timeEnd (a region when set), title, text and comma separated tags.
func annotationFrame(queryType string, rows []json.RawMessage) *data.Frame {
	var anns []annotation
	switch queryType {
	case "networkEvents":
		anns = eventAnnotations(rows)
	case "securityEvents":
		anns = securityAnnotations(rows)
	case "assuranceAlerts":
		anns = alertAnnotations(rows)
	}
	times := make([]time.Time, len(anns))
	ends := make([]*time.Time, len(anns))
	titles := make([]string, len(anns))
	texts := make([]string, len(anns))
	tags := make([]string, len(anns))
	for i, a := range anns {
		times[i], titles[i], texts[i] = a.time, a.title, a.text
		if !a.end.IsZero() {
			ends[i] = &a.end
		}
		tags[i] = strings.Join(a.tags, ",")
	}
	return data.NewFrame(queryType,
		data.NewField("time", nil, times),
		data.NewField("timeEnd", nil, ends),
		data.NewField("title", nil, titles),
		data.NewField("text", nil, texts),
		data.NewField("tags", nil, tags),
	)
}

func eventAnnotations(rows []json.RawMessage) []annotation {
	var out []annotation
	for _, raw := range rows {
		var e struct {
			OccurredAt        time.Time `json:"occurredAt"`
			Type              string    `json:"type"`
			Description       string    `json:"description"`
			Category          string    `json:"category"`
			DeviceName        string    `json:"deviceName"`
			ClientDescription string    `json:"clientDescription"`
		}
		if json.Unmarshal(raw, &e) != nil || e.OccurredAt.IsZero() {
			continue
		}
		out = append(out, annotation{
			time:  e.OccurredAt,
			title: e.Description,
			text:  joinNonEmpty(e.DeviceName, e.ClientDescription),
			tags:  nonEmpty(e.Category, e.Type),
		})
	}
	return out
}

func securityAnnotations(rows []json.RawMessage) []annotation {
	var out []annotation
	for _, raw := range rows {
		var e struct {
			TS        time.Time `json:"ts"`
			EventType string    `json:"eventType"`
			Message   string    `json:"message"`
			SrcIP     string    `json:"srcIp"`
			DestIP    string    `json:"destIp"`
			Blocked   bool      `json:"blocked"`
			Priority  string    `json:"priority"`
		}
		if json.Unmarshal(raw, &e) != nil || e.TS.IsZero() {
			continue
		}
		action := "allowed"
		if e.Blocked {
			action = "blocked"
		}
		text := action
		if e.SrcIP != "" || e.DestIP != "" {
			text = "From " + e.SrcIP + " to " + e.DestIP + ", " + action
		}
		tags := nonEmpty(e.EventType, action)
		if e.Priority != "" {
			tags = append(tags, "priority "+e.Priority)
		}
		out = append(out, annotation{time: e.TS, title: e.Message, text: text, tags: tags})
	}
	return out
}

// alertAnnotations reads alertRows output. A resolved alert is a region
// from start to resolution.
func alertAnnotations(rows []json.RawMessage) []annotation {
	var out []annotation
	for _, raw := range rows {
		var a struct {
			StartedAt   time.Time  `json:"startedAt"`
			ResolvedAt  *time.Time `json:"resolvedAt"`
			Severity    string     `json:"severity"`
			Title       string     `json:"title"`
			NetworkName string     `json:"networkName"`
			Devices     string     `json:"devices"`
			Description string     `json:"description"`
			Category    string     `json:"category"`
		}
		if json.Unmarshal(raw, &a) != nil || a.StartedAt.IsZero() {
			continue
		}
		ann := annotation{
			time:  a.StartedAt,
			title: a.Title,
			text:  joinNonEmpty(a.Description, a.NetworkName, a.Devices),
			tags:  nonEmpty(a.Severity, a.Category),
		}
		if a.ResolvedAt != nil {
			ann.end = *a.ResolvedAt
		}
		out = append(out, ann)
	}
	return out
}

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinNonEmpty(vals ...string) string { return strings.Join(nonEmpty(vals...), " · ") }
