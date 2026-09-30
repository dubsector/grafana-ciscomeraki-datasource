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

// Meraki sends rel=next unquoted; RFC 8288 also allows rel="next".
var reLinkNext = regexp.MustCompile(`<([^>]+)>;\s*rel=(?:next|"next")\s*(?:[,;]|$)`)

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
	return body, nextLink(resp.Header.Get("Link")), resp.StatusCode, retryAfter, nil
}

// nextLink returns the rel=next URL from a Link header, or "" on the last page.
func nextLink(header string) string {
	if m := reLinkNext.FindStringSubmatch(header); len(m) == 2 {
		return m[1]
	}
	return ""
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
	u := c.base + fmt.Sprintf("/networks/%s/events", networkID) + "?" + p.Encode()
	// The event log always sends rel=next, even past the newest event, so the
	// generic get() loop would never end. Stop on an empty page or once past t1.
	var out []json.RawMessage
	for page := 0; page < maxEventPages; page++ {
		body, next, err := c.fetch(ctx, u)
		if err != nil {
			return nil, err
		}
		var w struct {
			PageEndAt string            `json:"pageEndAt"`
			Events    []json.RawMessage `json:"events"`
		}
		if e := json.Unmarshal(body, &w); e != nil || w.Events == nil {
			return append(out, json.RawMessage(body)), nil
		}
		// Pages come newest first; reverse so the result is oldest first.
		for i := len(w.Events) - 1; i >= 0; i-- {
			out = append(out, w.Events[i])
		}
		if len(w.Events) == 0 || next == "" || next == u || pastEnd(w.PageEndAt, t1) {
			break
		}
		u = next
	}
	return out, nil
}

// maxEventPages caps one events query at 100k events to protect the org's rate budget.
const maxEventPages = 100

// pastEnd reports whether a page's pageEndAt has reached t1.
func pastEnd(pageEndAt string, t1 time.Time) bool {
	if t1.IsZero() {
		return false
	}
	end, err := time.Parse(time.RFC3339Nano, pageEndAt)
	return err == nil && !end.Before(t1)
}

func (c *Client) SecurityEvents(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"perPage": {"1000"}, "t0": {ts(t0)}, "t1": {ts(t1)}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/appliance/security/events", networkID), p)
}

func (c *Client) NetworkClients(ctx context.Context, networkID string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/networks/%s/clients", networkID), url.Values{"perPage": {"1000"}})
}

func (c *Client) DeviceClients(ctx context.Context, serial string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/devices/%s/clients", serial), nil)
}

// Meraki has no org-wide version of the wireless latency and connection
// stats endpoints, so both need a network.
func (c *Client) WirelessLatencyStats(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/wireless/latencyStats", networkID), p)
}

func (c *Client) WirelessConnectionStats(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/wireless/connectionStats", networkID), p)
}

func (c *Client) WirelessClientCount(ctx context.Context, networkID string, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}, "resolution": {"300"}}
	return c.get(ctx, fmt.Sprintf("/networks/%s/wireless/clientCountHistory", networkID), p)
}

func (c *Client) SwitchPortStatuses(ctx context.Context, serial string) ([]json.RawMessage, error) {
	return c.get(ctx, fmt.Sprintf("/devices/%s/switch/ports/statuses", serial), nil)
}

// ApplianceUplinkStatuses covers the whole org when networkID is empty.
func (c *Client) ApplianceUplinkStatuses(ctx context.Context, networkID string) ([]json.RawMessage, error) {
	var p url.Values
	if networkID != "" {
		p = url.Values{"networkIds[]": {networkID}}
	}
	return c.get(ctx, fmt.Sprintf("/organizations/%s/appliance/uplink/statuses", c.orgID), p)
}

func (c *Client) VPNStats(ctx context.Context, t0, t1 time.Time) ([]json.RawMessage, error) {
	p := url.Values{"t0": {ts(t0)}, "t1": {ts(t1)}, "perPage": {"300"}}
	return c.get(ctx, fmt.Sprintf("/organizations/%s/appliance/vpn/stats", c.orgID), p)
}
