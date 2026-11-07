package meraki

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNetworksSendsCredentialsAndQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Cisco-Meraki-API-Key"); got != "secret" {
			t.Errorf("API key header = %q", got)
		}
		if got := r.URL.Path; got != "/organizations/org-1/networks" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.Query().Get("perPage"); got != "1000" {
			t.Errorf("perPage = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"network-1"}]`))
	}))
	defer server.Close()

	rows, err := New(server.URL, "org-1", "secret").Networks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || string(rows[0]) != `{"id":"network-1"}` {
		t.Fatalf("rows = %s", rows)
	}
}

func TestGetFollowsPagination(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first := "<" + server.URL + "/organizations?startingAfter=0000000000>; rel=first"
		last := "<" + server.URL + "/organizations?endingBefore=zzzzzzzzzz>; rel=last"
		if r.URL.Query().Get("page") == "2" {
			w.Header().Set("Link", first+", <"+server.URL+"/organizations?endingBefore=2>; rel=prev, "+last)
			_, _ = w.Write([]byte(`[{"id":2}]`))
			return
		}
		// Same shape as a real Meraki response: unquoted rel values.
		w.Header().Set("Link", first+", <"+server.URL+"/organizations?page=2>; rel=next, "+last)
		_, _ = w.Write([]byte(`[{"id":1}]`))
	}))
	defer server.Close()

	rows, err := New(server.URL, "", "").Organizations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d", len(rows))
	}
}

func TestNextLink(t *testing.T) {
	t.Parallel()

	const base = "https://api.meraki.com/api/v1/organizations/1/devices"
	cases := map[string]struct{ header, want string }{
		"meraki unquoted": {
			header: "<" + base + "?perPage=3&startingAfter=0000000000>; rel=first, <" + base + "?perPage=3&startingAfter=Q2XX-AAAA-BBBB>; rel=next, <" + base + "?endingBefore=zzzzzzzzzz&perPage=3>; rel=last",
			want:   base + "?perPage=3&startingAfter=Q2XX-AAAA-BBBB",
		},
		"quoted":           {header: "<" + base + "?page=2>; rel=\"next\"", want: base + "?page=2"},
		"next listed last": {header: "<" + base + "?a=1>; rel=prev, <" + base + "?b=2>; rel=next", want: base + "?b=2"},
		"last page":        {header: "<" + base + "?a=1>; rel=first, <" + base + "?b=2>; rel=prev, <" + base + "?c=3>; rel=last"},
		"no header":        {},
	}
	for name, tc := range cases {
		if got := nextLink(tc.header); got != tc.want {
			t.Errorf("%s: nextLink = %q, want %q", name, got, tc.want)
		}
	}
}

func TestNetworkEventsUnwrapsEvents(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"pageStartAt":"now","events":[{"type":"client"}]}`))
	}))
	defer server.Close()

	rows, err := New(server.URL, "", "").NetworkEvents(context.Background(), "network-1", "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d", len(rows))
	}
	var event map[string]string
	if err := json.Unmarshal(rows[0], &event); err != nil {
		t.Fatal(err)
	}
	if event["type"] != "client" {
		t.Fatalf("event = %v", event)
	}
}

// The real event log always sends rel=next, so paging must stop on its own.
func TestNetworkEventsStopsPaging(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		pages []string
		want  []string
		calls int
	}{
		{
			name: "empty page",
			pages: []string{
				`{"pageEndAt":"2026-09-28T10:00:00.000000Z","events":[{"type":"b"},{"type":"a"}]}`,
				`{"pageEndAt":"2026-09-28T12:00:00.000000Z","events":[{"type":"c"}]}`,
				`{"pageEndAt":"2026-09-29T00:00:00.000000Z","events":[]}`,
			},
			want:  []string{"a", "b", "c"},
			calls: 3,
		},
		{
			name: "reached t1",
			pages: []string{
				`{"pageEndAt":"2026-09-29T00:00:00.000000Z","events":[{"type":"b"},{"type":"a"}]}`,
				`{"pageEndAt":"2026-09-29T01:00:00.000000Z","events":[{"type":"z"}]}`,
			},
			want:  []string{"a", "b"},
			calls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				page := tc.pages[min(calls, len(tc.pages)-1)]
				calls++
				w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?startingAfter=p"+string(rune('0'+calls))+">; rel=next")
				_, _ = w.Write([]byte(page))
			}))
			defer server.Close()

			rows, err := New(server.URL, "", "k").NetworkEvents(context.Background(), "network-1", "wireless", t0, t1)
			if err != nil {
				t.Fatal(err)
			}
			if calls != tc.calls {
				t.Fatalf("calls = %d, want %d", calls, tc.calls)
			}
			var got []string
			for _, r := range rows {
				var e map[string]string
				if err := json.Unmarshal(r, &e); err != nil {
					t.Fatal(err)
				}
				got = append(got, e["type"])
			}
			if len(got) != len(tc.want) {
				t.Fatalf("events = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("events = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestOrganizationsReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	if _, err := New(server.URL, "", "bad-key").Organizations(context.Background()); err == nil {
		t.Fatal("expected an HTTP error")
	}
}
