package meraki

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Regenerate with: go test ./pkg/meraki -run TestClientMatchesSpec -update
var update = flag.Bool("update", false, "refresh testdata/meraki-spec.json from the published Meraki OpenAPI spec")

const (
	specURL  = "https://raw.githubusercontent.com/meraki/openapi/master/openapi/spec3.json"
	specFile = "testdata/meraki-spec.json"
)

// specOp is the part of a GET operation the client has to agree with.
type specOp struct {
	Params  []string `json:"params"`
	PerPage *[2]int  `json:"perPage,omitempty"`
}

type specData struct {
	Version string            `json:"version"`
	Get     map[string]specOp `json:"get"`
}

func TestClientMatchesSpec(t *testing.T) {
	if *update {
		updateSpec(t)
	}
	spec := loadSpec(t)

	var (
		mu   sync.Mutex
		reqs []*url.URL
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r.URL)
		mu.Unlock()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	ctx := context.Background()
	c := New(server.URL, "org-1", "key")
	t1 := time.Now()
	t0 := t1.Add(-time.Hour)

	calls := map[string]func() error{
		"Organizations":        func() error { _, err := c.Organizations(ctx); return err },
		"Networks":             func() error { _, err := c.Networks(ctx); return err },
		"Devices":              func() error { _, err := c.Devices(ctx); return err },
		"DeviceAvailabilities": func() error { _, err := c.DeviceAvailabilities(ctx, "wireless", "N_1"); return err },
		"DeviceAvailabilityHistory": func() error {
			_, err := c.DeviceAvailabilityHistory(ctx, t0, t1, "wireless", "N_1")
			return err
		},
		"DeviceStatuses":          func() error { _, err := c.DeviceStatuses(ctx, "switch", "N_1"); return err },
		"NetworkEvents":           func() error { _, err := c.NetworkEvents(ctx, "N_1", "wireless", t0, t1); return err },
		"SecurityEvents":          func() error { _, err := c.SecurityEvents(ctx, "N_1", t0, t1); return err },
		"NetworkClients":          func() error { _, err := c.NetworkClients(ctx, "N_1"); return err },
		"DeviceClients":           func() error { _, err := c.DeviceClients(ctx, "Q2XX-XXXX-XXXX"); return err },
		"WirelessLatencyStats":    func() error { _, err := c.WirelessLatencyStats(ctx, "N_1", t0, t1); return err },
		"WirelessConnectionStats": func() error { _, err := c.WirelessConnectionStats(ctx, "N_1", t0, t1); return err },
		"WirelessClientCount":     func() error { _, err := c.WirelessClientCount(ctx, "N_1", t0, t1); return err },
		"SwitchPortStatuses":      func() error { _, err := c.SwitchPortStatuses(ctx, "Q2XX-XXXX-XXXX"); return err },
		"ApplianceUplinkStatuses": func() error { _, err := c.ApplianceUplinkStatuses(ctx, "N_1"); return err },
		"VPNStats":                func() error { _, err := c.VPNStats(ctx, t0, t1, "N_1"); return err },
		"ApplianceLANPorts":       func() error { _, err := c.ApplianceLANPorts(ctx, "N_1"); return err },
	}

	ct := reflect.TypeOf(c)
	for i := range ct.NumMethod() {
		if name := ct.Method(i).Name; calls[name] == nil {
			t.Errorf("Client.%s has no entry in this test; add one so its endpoint is checked", name)
		}
	}

	for name, call := range calls {
		mu.Lock()
		reqs = nil
		mu.Unlock()
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		mu.Lock()
		got := reqs
		mu.Unlock()
		if len(got) == 0 {
			t.Errorf("%s made no request", name)
		}
		for _, u := range got {
			checkRequest(t, spec, name, u)
		}
	}
}

func checkRequest(t *testing.T, spec specData, method string, u *url.URL) {
	t.Helper()
	tmpl, op, ok := spec.match(u.Path)
	if !ok {
		t.Errorf("%s: GET %s is not in Meraki API spec %s", method, u.Path, spec.Version)
		return
	}
	allowed := map[string]bool{}
	for _, p := range op.Params {
		allowed[p] = true
	}
	for key, vals := range u.Query() {
		name := strings.TrimSuffix(key, "[]")
		if !allowed[name] {
			t.Errorf("%s: %s has no query parameter %q", method, tmpl, key)
			continue
		}
		if name == "perPage" && op.PerPage != nil {
			n, err := strconv.Atoi(vals[0])
			if err != nil || n < op.PerPage[0] || n > op.PerPage[1] {
				t.Errorf("%s: perPage=%s is outside %d-%d for %s", method, vals[0], op.PerPage[0], op.PerPage[1], tmpl)
			}
		}
	}
}

// match finds the spec path for a concrete request path, preferring the
// template with the most literal segments.
func (s specData) match(path string) (string, specOp, bool) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	best, bestLiterals := "", -1
	for tmpl := range s.Get {
		tsegs := strings.Split(strings.Trim(tmpl, "/"), "/")
		if len(tsegs) != len(segs) {
			continue
		}
		literals := 0
		for i, ts := range tsegs {
			if strings.HasPrefix(ts, "{") && strings.HasSuffix(ts, "}") {
				continue
			}
			if ts != segs[i] {
				literals = -1
				break
			}
			literals++
		}
		if literals > bestLiterals {
			best, bestLiterals = tmpl, literals
		}
	}
	if best == "" {
		return "", specOp{}, false
	}
	return best, s.Get[best], true
}

func loadSpec(t *testing.T) specData {
	t.Helper()
	b, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	var s specData
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

var perPageRange = regexp.MustCompile(`Acceptable range is (\d+) - (\d+)`)

// updateSpec reduces the full OpenAPI spec (~8 MB) to GET paths and their
// query parameters, one path per line so diffs stay readable.
func updateSpec(t *testing.T) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, specURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch spec: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Info  struct{ Version string } `json:"info"`
		Paths map[string]struct {
			Get *struct {
				Parameters []struct {
					Name        string `json:"name"`
					In          string `json:"in"`
					Description string `json:"description"`
				} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}

	paths := make([]string, 0, len(raw.Paths))
	ops := map[string]specOp{}
	for p, item := range raw.Paths {
		if item.Get == nil {
			continue
		}
		op := specOp{Params: []string{}}
		for _, prm := range item.Get.Parameters {
			if prm.In != "query" {
				continue
			}
			op.Params = append(op.Params, prm.Name)
			if m := perPageRange.FindStringSubmatch(prm.Description); prm.Name == "perPage" && m != nil {
				lo, _ := strconv.Atoi(m[1])
				hi, _ := strconv.Atoi(m[2])
				op.PerPage = &[2]int{lo, hi}
			}
		}
		sort.Strings(op.Params)
		ops[p] = op
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	version, _ := json.Marshal(raw.Info.Version)
	buf.WriteString("{\n  \"version\": " + string(version) + ",\n  \"get\": {\n")
	for i, p := range paths {
		k, _ := json.Marshal(p)
		v, _ := json.Marshal(ops[p])
		buf.WriteString("    " + string(k) + ": " + string(v))
		if i < len(paths)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  }\n}\n")
	if err := os.WriteFile(specFile, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
