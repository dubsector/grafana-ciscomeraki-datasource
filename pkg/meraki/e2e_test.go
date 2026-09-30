//go:build e2e

package meraki

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// Runs every Client method against meraki-api-emulator:
//
//	MERAKI_E2E_URL=http://127.0.0.1:8765/api/v1 MERAKI_E2E_NOW=<emulator --now> \
//	  go test -tags e2e ./pkg/meraki -run TestE2E

// callTimeout catches paging loops that never end. Calls take milliseconds.
const callTimeout = 10 * time.Second

// pager overrides perPage on requests that send one and counts requests.
type pager struct {
	perPage  string
	requests atomic.Int64
	paged    atomic.Bool
}

func (p *pager) RoundTrip(r *http.Request) (*http.Response, error) {
	p.requests.Add(1)
	if q := r.URL.Query(); p.perPage != "" && q.Has("perPage") {
		p.paged.Store(true)
		r = r.Clone(r.Context())
		q.Set("perPage", p.perPage)
		r.URL.RawQuery = q.Encode()
	}
	return http.DefaultTransport.RoundTrip(r)
}

type e2eStats struct {
	items        int
	sendsPerPage bool
	multiPage    bool
}

type e2eRun struct {
	base  string
	stats map[string]*e2eStats
}

type fetchFunc func(context.Context, *Client) ([]json.RawMessage, error)

// check runs fn with the client's own perPage, then again with a perPage that
// splits the result into about ten pages. Both runs must return the same items.
func (e *e2eRun) check(t *testing.T, orgID, method string, fn fetchFunc) []json.RawMessage {
	t.Helper()
	full, _ := e.fetch(t, orgID, method, "", fn)
	size := min(max(3, (len(full)+9)/10), 300)
	paged, p := e.fetch(t, orgID, method, strconv.Itoa(size), fn)

	s := e.stats[method]
	if s == nil {
		s = &e2eStats{}
		e.stats[method] = s
	}
	s.items += len(full)
	s.sendsPerPage = s.sendsPerPage || p.paged.Load()
	s.multiPage = s.multiPage || (p.paged.Load() && p.requests.Load() > 1)

	if len(paged) != len(full) {
		t.Errorf("%s: %d items with perPage=%d, %d without", method, len(paged), size, len(full))
		return full
	}
	for i := range full {
		if !bytes.Equal(paged[i], full[i]) {
			t.Errorf("%s: item %d differs with perPage=%d:\n%s\n%s", method, i, size, paged[i], full[i])
			break
		}
	}
	return full
}

func (e *e2eRun) fetch(t *testing.T, orgID, method, perPage string, fn fetchFunc) ([]json.RawMessage, *pager) {
	t.Helper()
	p := &pager{perPage: perPage}
	c := New(e.base, orgID, "e2e")
	c.http.Transport = p
	ctx, cancel := context.WithTimeout(t.Context(), callTimeout)
	defer cancel()
	rows, err := fn(ctx, c)
	if err != nil {
		t.Fatalf("%s (perPage=%q): %v", method, perPage, err)
	}
	return rows, p
}

func decode[T any](t *testing.T, rows []json.RawMessage) []T {
	t.Helper()
	out := make([]T, len(rows))
	for i, r := range rows {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			t.Fatalf("decode %s: %v", r, err)
		}
	}
	return out
}

// checkEventOrder wants events oldest first and inside (t0, t1).
func checkEventOrder(t *testing.T, rows []json.RawMessage, t0, t1 time.Time) {
	t.Helper()
	prev := t0
	for i, ev := range decode[struct{ OccurredAt time.Time }](t, rows) {
		at := ev.OccurredAt
		if at.Before(prev) || !at.After(t0) || !at.Before(t1) {
			t.Errorf("event %d at %s is out of order or outside %s..%s", i, at, t0, t1)
			return
		}
		prev = at
	}
}

func TestE2E(t *testing.T) {
	base := os.Getenv("MERAKI_E2E_URL")
	if base == "" {
		t.Fatal("MERAKI_E2E_URL is not set")
	}
	now, err := time.Parse(time.RFC3339, os.Getenv("MERAKI_E2E_NOW"))
	if err != nil {
		t.Fatalf("MERAKI_E2E_NOW: %v", err)
	}
	day0, week0 := now.Add(-day), now.Add(-7*day)
	e := &e2eRun{base: base, stats: map[string]*e2eStats{}}

	type org struct{ ID, Name string }
	type network struct {
		ID, Name     string
		ProductTypes []string
	}
	type device struct{ Serial, ProductType string }

	orgs := e.check(t, "", "Organizations", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
		return c.Organizations(ctx)
	})
	for _, o := range decode[org](t, orgs) {
		t.Run(o.Name, func(t *testing.T) {
			run := func(method string, fn fetchFunc) []json.RawMessage {
				t.Helper()
				return e.check(t, o.ID, method, fn)
			}
			nets := decode[network](t, run("Networks", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
				return c.Networks(ctx)
			}))
			devs := decode[device](t, run("Devices", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
				return c.Devices(ctx)
			}))

			products := []string{""}
			for _, d := range devs {
				if !slices.Contains(products, d.ProductType) {
					products = append(products, d.ProductType)
				}
			}
			for _, pt := range products {
				run("DeviceAvailabilities", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
					return c.DeviceAvailabilities(ctx, pt)
				})
			}
			run("DeviceAvailabilityHistory", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
				return c.DeviceAvailabilityHistory(ctx, week0, now, "")
			})
			run("ApplianceUplinkStatuses", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
				return c.ApplianceUplinkStatuses(ctx)
			})
			run("VPNStats", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
				return c.VPNStats(ctx, day0, now)
			})

			for _, n := range nets {
				t.Run(n.Name, func(t *testing.T) {
					run := func(method string, fn fetchFunc) []json.RawMessage {
						t.Helper()
						return e.check(t, o.ID, method, fn)
					}
					run("NetworkClients", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
						return c.NetworkClients(ctx, n.ID)
					})
					for _, pt := range n.ProductTypes {
						events := run("NetworkEvents", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
							return c.NetworkEvents(ctx, n.ID, pt, day0, now)
						})
						checkEventOrder(t, events, day0, now)
					}
					if slices.Contains(n.ProductTypes, "appliance") {
						run("SecurityEvents", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
							return c.SecurityEvents(ctx, n.ID, day0, now)
						})
					}
					if slices.Contains(n.ProductTypes, "wireless") {
						run("WirelessLatencyStats", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
							return c.WirelessLatencyStats(ctx, n.ID, day0, now)
						})
						run("WirelessConnectionStats", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
							return c.WirelessConnectionStats(ctx, n.ID, day0, now)
						})
						run("WirelessClientCount", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
							return c.WirelessClientCount(ctx, n.ID, day0, now)
						})
					}
				})
			}

			for _, d := range devs {
				run("DeviceClients", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
					return c.DeviceClients(ctx, d.Serial)
				})
				if d.ProductType == "switch" {
					run("SwitchPortStatuses", func(ctx context.Context, c *Client) ([]json.RawMessage, error) {
						return c.SwitchPortStatuses(ctx, d.Serial)
					})
				}
			}
		})
	}

	// Every method must run, return data, and page at least once if it can.
	ct := reflect.TypeFor[*Client]()
	for i := range ct.NumMethod() {
		name := ct.Method(i).Name
		s := e.stats[name]
		switch {
		case s == nil:
			t.Errorf("Client.%s has no call in this test; add one", name)
		case s.items == 0:
			t.Errorf("Client.%s returned nothing from the emulator", name)
		case s.sendsPerPage && !s.multiPage:
			t.Errorf("Client.%s never needed a second page", name)
		}
	}
}
