package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/dubsector/meraki-datasource/pkg/meraki"
)

var (
	_ backend.QueryDataHandler      = (*DS)(nil)
	_ backend.CheckHealthHandler    = (*DS)(nil)
	_ backend.CallResourceHandler   = (*DS)(nil)
	_ instancemgmt.InstanceDisposer = (*DS)(nil)
)

type DS struct{ c *meraki.Client }

type config struct {
	BaseURL        string `json:"baseUrl"`
	OrganizationID string `json:"organizationId"`
}

func NewDS(_ context.Context, s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var cfg config
	if err := json.Unmarshal(s.JSONData, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &DS{c: meraki.New(cfg.BaseURL, cfg.OrganizationID, s.DecryptedSecureJSONData["apiKey"])}, nil
}

func (d *DS) Dispose() {}

// ── Health ────────────────────────────────────────────────────────────────────

func (d *DS) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if _, err := d.c.Organizations(ctx); err != nil {
		return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: err.Error()}, nil
	}
	return &backend.CheckHealthResult{Status: backend.HealthStatusOk, Message: "Connected to Meraki API"}, nil
}

// ── Resources (dropdown data) ─────────────────────────────────────────────────

func (d *DS) CallResource(ctx context.Context, req *backend.CallResourceRequest, send backend.CallResourceResponseSender) error {
	var rows []json.RawMessage
	var err error
	switch req.Path {
	case "networks":
		rows, err = d.c.Networks(ctx)
	case "devices":
		rows, err = d.c.Devices(ctx)
	default:
		return send.Send(&backend.CallResourceResponse{Status: http.StatusNotFound})
	}
	if err != nil {
		return send.Send(&backend.CallResourceResponse{Status: http.StatusInternalServerError, Body: []byte(err.Error())})
	}
	b, _ := json.Marshal(rows)
	return send.Send(&backend.CallResourceResponse{
		Status:  http.StatusOK,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    b,
	})
}

// ── Queries ───────────────────────────────────────────────────────────────────

type query struct {
	QueryType    string `json:"queryType"`
	NetworkID    string `json:"networkId"`
	DeviceSerial string `json:"deviceSerial"`
	ProductType  string `json:"productType"`
	Historical   bool   `json:"historical"`
}

func (d *DS) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		resp.Responses[q.RefID] = d.run(ctx, q)
	}
	return resp, nil
}

func (d *DS) run(ctx context.Context, bq backend.DataQuery) backend.DataResponse {
	var q query
	if err := json.Unmarshal(bq.JSON, &q); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	t0, t1 := bq.TimeRange.From, bq.TimeRange.To

	rows, err := d.dispatch(ctx, q, t0, t1)
	if err != nil {
		log.DefaultLogger.Error("meraki query", "type", q.QueryType, "err", err)
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	frame, err := buildFrame(q.QueryType, rows)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}
	return backend.DataResponse{Frames: data.Frames{frame}}
}

func (d *DS) dispatch(ctx context.Context, q query, t0, t1 time.Time) ([]json.RawMessage, error) {
	switch q.QueryType {
	case "deviceAvailabilities":
		if q.Historical {
			return d.c.DeviceAvailabilityHistory(ctx, t0, t1, q.ProductType)
		}
		return d.c.DeviceAvailabilities(ctx, q.ProductType)
	case "networkEvents":
		if q.NetworkID == "" {
			return nil, fmt.Errorf("networkId required for networkEvents")
		}
		return d.c.NetworkEvents(ctx, q.NetworkID, q.ProductType, t0, t1)
	case "securityEvents":
		if q.NetworkID == "" {
			return nil, fmt.Errorf("networkId required for securityEvents")
		}
		return d.c.SecurityEvents(ctx, q.NetworkID, t0, t1)
	case "networkClients":
		if q.NetworkID == "" {
			return nil, fmt.Errorf("networkId required for networkClients")
		}
		return d.c.NetworkClients(ctx, q.NetworkID)
	case "deviceClients":
		if q.DeviceSerial == "" {
			return nil, fmt.Errorf("deviceSerial required for deviceClients")
		}
		return d.c.DeviceClients(ctx, q.DeviceSerial)
	case "wirelessLatencyStats":
		return d.c.WirelessLatencyStats(ctx, q.NetworkID, t0, t1)
	case "wirelessConnectionStats":
		return d.c.WirelessConnectionStats(ctx, q.NetworkID, t0, t1)
	case "wirelessClientCount":
		if q.NetworkID == "" {
			return nil, fmt.Errorf("networkId required for wirelessClientCount")
		}
		return d.c.WirelessClientCount(ctx, q.NetworkID, t0, t1)
	case "switchPortStatuses":
		if q.DeviceSerial == "" {
			return nil, fmt.Errorf("deviceSerial required for switchPortStatuses")
		}
		return d.c.SwitchPortStatuses(ctx, q.DeviceSerial)
	case "applianceUplinkStatuses":
		return d.c.ApplianceUplinkStatuses(ctx)
	case "vpnStats":
		return d.c.VPNStats(ctx, t0, t1)
	default:
		return nil, fmt.Errorf("unknown queryType: %s", q.QueryType)
	}
}

// ── Frame builder ─────────────────────────────────────────────────────────────

var timeFieldNames = map[string]bool{
	"ts": true, "occurredAt": true, "startTs": true, "endTs": true,
	"lastReportedAt": true, "updatedAt": true, "createdAt": true, "lastSeenAt": true,
}

func buildFrame(name string, rows []json.RawMessage) (*data.Frame, error) {
	frame := data.NewFrame(name)
	if len(rows) == 0 {
		return frame, nil
	}

	var keys []string
	seen := map[string]bool{}
	maps := make([]map[string]interface{}, 0, len(rows))

	for _, raw := range rows {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		flat := flatObj(obj, "")
		maps = append(maps, flat)
		for k := range flat {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}

	cols := map[string][]interface{}{}
	for _, k := range keys {
		cols[k] = make([]interface{}, len(maps))
	}
	for i, m := range maps {
		for _, k := range keys {
			cols[k][i] = m[k]
		}
	}
	for _, k := range keys {
		frame.Fields = append(frame.Fields, makeField(k, cols[k]))
	}
	return frame, nil
}

func flatObj(m map[string]interface{}, prefix string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if child, ok := v.(map[string]interface{}); ok {
			for ck, cv := range flatObj(child, key) {
				out[ck] = cv
			}
		} else {
			out[key] = v
		}
	}
	return out
}

func makeField(name string, vals []interface{}) *data.Field {
	for _, v := range vals {
		if v == nil {
			continue
		}
		switch v.(type) {
		case float64:
			typed := make([]*float64, len(vals))
			for i, x := range vals {
				if f, ok := x.(float64); ok {
					typed[i] = &f
				}
			}
			return data.NewField(name, nil, typed)
		case bool:
			typed := make([]*bool, len(vals))
			for i, x := range vals {
				if b, ok := x.(bool); ok {
					typed[i] = &b
				}
			}
			return data.NewField(name, nil, typed)
		case string:
			if timeFieldNames[name] {
				typed := make([]*time.Time, len(vals))
				for i, x := range vals {
					if s, ok := x.(string); ok && s != "" {
						if t, err := time.Parse(time.RFC3339, s); err == nil {
							typed[i] = &t
						}
					}
				}
				return data.NewField(name, nil, typed)
			}
			typed := make([]*string, len(vals))
			for i, x := range vals {
				if s, ok := x.(string); ok {
					typed[i] = &s
				}
			}
			return data.NewField(name, nil, typed)
		}
		break
	}
	typed := make([]*string, len(vals))
	for i, x := range vals {
		if x != nil {
			s := fmt.Sprintf("%v", x)
			typed[i] = &s
		}
	}
	return data.NewField(name, nil, typed)
}
