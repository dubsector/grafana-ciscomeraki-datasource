package meraki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.meraki.com/api/v1"

var reLinkNext = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

type Client struct {
	base   string
	orgID  string
	apiKey string
	http   *http.Client
}

func New(baseURL, orgID, apiKey string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		base:   strings.TrimRight(baseURL, "/"),
		orgID:  orgID,
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// get issues a GET and follows RFC-5988 Link pagination automatically.
func (c *Client) get(ctx context.Context, path string, p url.Values) ([]json.RawMessage, error) {
	u := c.base + path
	if len(p) > 0 {
		u += "?" + p.Encode()
	}
	var out []json.RawMessage
	for {
		body, next, err := c.fetch(ctx, u)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 && body[0] == '[' {
			var page []json.RawMessage
			if err := json.Unmarshal(body, &page); err != nil {
				return nil, err
			}
			out = append(out, page...)
		} else {
			out = append(out, json.RawMessage(body))
		}
		if next == "" {
			break
		}
		u = next
	}
	return out, nil
}

// fetch issues one request with retry on 429/5xx.
func (c *Client) fetch(ctx context.Context, u string) ([]byte, string, error) {
	delay := 500 * time.Millisecond
	for attempt := 0; attempt <= 3; attempt++ {
		body, next, status, retryAfter, err := c.do(ctx, u)
		if err != nil {
			return nil, "", err
		}
		switch {
		case status == http.StatusTooManyRequests:
			wait := retryAfter
			if wait <= 0 {
				wait = delay
				delay *= 2
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		case status >= 500:
			if attempt == 3 {
				return nil, "", fmt.Errorf("meraki: HTTP %d after retries", status)
			}
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
			delay *= 2
		case status >= 400:
			return nil, "", fmt.Errorf("meraki: HTTP %d — %s", status, string(body))
		default:
			return body, next, nil
		}
	}
	return nil, "", fmt.Errorf("meraki: max retries for %s", u)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, next string, status int, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", 0, 0, err
	}
	req.Header.Set("X-Cisco-Meraki-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", 0, 0, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", resp.StatusCode, 0, err
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if s, e := strconv.Atoi(ra); e == nil {
			retryAfter = time.Duration(s) * time.Second
		}
	}
	if link := resp.Header.Get("Link"); link != "" {
		if m := reLinkNext.FindStringSubmatch(link); len(m) == 2 {
			next = m[1]
		}
	}
	return body, next, resp.StatusCode, retryAfter, nil
}

// ts formats a time.Time as RFC3339 UTC.
func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ── Public methods ────────────────────────────────────────────────────────────

func (c *Client) Organizations(ctx context.Context) ([]json.RawMessage, error) {
	return c.get(ctx, "/organizations", nil)
}

func (c *Client) Networks(ctx context.Context) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/organizations/%s/networks", c.orgID), url.Values{"perPage": {"1000"}})
}

func (c *Client) Devices(ctx context.Context) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/organizations/%s/devices", c.orgID), url.Values{"perPage": {"1000"}})
}

func (c *Client) DeviceAvailabilities(ctx context.Context, productType string) ([]json.RawMessage, error) {
	p := url.Values{"perPage": {"1000"}}
	if productType != "" {
		p.Set("productTypes[]", productType)
	}
	return c.get(ctx, fmt.Sprintf("/organizations/%s/devices/availabilities", c.orgID), p)
}

func (c *Client) DeviceAvailabilityHistory(ctx context.Context, t0, t1 time.Time, productType string) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}, "perPage": {"1000"}}
	if productType != "" {
		p.Set("productTypes[]", productType)
	}
	return c.get(ctx, fmt.Sprintf("/organizations/%s/devices/availabilities/changeHistory", c.orgID), p)
}

func (c *Client) NetworkEvents(ctx context.Context, networkID, productType string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"perPage": {"1000"}}
	if productType != "" {
		p.Set("productType", productType)
	}
	if !t0.IsZero() {
		p.Set("startingAfter", ts(t0))
	}
	if !t1.IsZero() {
		p.Set("endingBefore", ts(t1))
	}
	raw, err := c.get(ctx, fmt.Sprintf("/networks/%s/events", networkID), p)
	if err != nil {
		return nil, err
	}
	// Response shape: { "pageStartAt": "...", "events": [...] }
	var out []json.RawMessage
	for _, r := range raw {
		var w struct {
			Events []json.RawMessage `json:"events"`
		}
		if e := json.Unmarshal(r, &w); e == nil && w.Events != nil {
			out = append(out, w.Events...)
		} else {
			out = append(out, r)
		}
	}
	return out, nil
}

func (c *Client) SecurityEvents(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"perPage": {"1000"}, "t0": {ts(t0)}, "t1": {ts(t1)}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/appliance/security/intrusion/security/events", networkID), p)
}

func (c *Client) NetworkClients(ctx context.Context, networkID string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/networks/%s/clients", networkID), url.Values{"perPage": {"1000"}})
}

func (c *Client) DeviceClients(ctx context.Context, serial string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/devices/%s/clients", serial), nil)
}

func (c *Client) WirelessLatencyStats(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}}
	path := fmt.Sprintf("/organizations/%s/wireless/devices/latencyStats", c.orgID)
	if networkID != "" {
		path = fmt.Sprintf("/networks/%s/wireless/latencyStats", networkID)
	}
	return c.get(ctx, path, p)
}

func (c *Client) WirelessConnectionStats(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}}
	path := fmt.Sprintf("/organizations/%s/wireless/devices/connectionStats", c.orgID)
	if networkID != "" {
		path = fmt.Sprintf("/networks/%s/wireless/connectionStats", networkID)
	}
	return c.get(ctx, path, p)
}

func (c *Client) WirelessClientCount(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}, "resolution": {"300"}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/wireless/clientCountHistory", networkID), p)
}

func (c *Client) SwitchPortStatuses(ctx context.Context, serial string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/devices/%s/switch/ports/statuses", serial), nil)
}

func (c *Client) ApplianceUplinkStatuses(ctx context.Context) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/organizations/%s/appliance/uplink/statuses", c.orgID), nil)
}

func (c *Client) VPNStats(ctx context.Context, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}, "perPage": {"1000"}}
	return c.get(ctx, fmt.Sprintf("/organizations/%s/appliance/vpn/stats", c.orgID), p)
}
