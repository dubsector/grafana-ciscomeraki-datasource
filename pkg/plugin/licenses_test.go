package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dubsector/meraki-datasource/pkg/meraki"
)

func TestLicenseRowsCoverBothModelsAndFailures(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/organizations/1/licenses/overview":
			_, _ = w.Write([]byte(`{"status":"OK","expirationDate":"Oct 15, 2026 UTC","licensedDeviceCounts":{"MR":3,"MX68":1}}`))
		case "/organizations/2/licenses/overview":
			_, _ = w.Write([]byte(`{"licenseCount":5,"states":{"active":{"count":3},"expired":{"count":0},"expiring":{"count":1},"unused":{"count":1}}}`))
		default:
			http.Error(w, `{"errors":["API is disabled"]}`, http.StatusForbidden)
		}
	}))
	t.Cleanup(server.Close)

	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	raw := licenseRows(context.Background(), meraki.New(server.URL, "1", "key"),
		[]orgRef{{"3", "Gamma"}, {"2", "Beta"}, {"1", "Alpha"}}, now)
	var rows []licenseRow
	for _, r := range raw {
		var l licenseRow
		if err := json.Unmarshal(r, &l); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, l)
	}
	if len(rows) != 3 || rows[0].Organization != "Alpha" {
		t.Fatalf("rows = %+v, want Alpha (expiring soonest) first", rows)
	}
	if a := rows[0]; a.Model != "Co-termination" || *a.DaysLeft != 29 || *a.LicensedDevices != 4 {
		t.Errorf("co-termination row = %+v", a)
	}
	if b := rows[1]; b.Model != "Per-device" || b.Status != "Licenses expiring" || *b.Active != 3 || *b.Unused != 1 {
		t.Errorf("per-device row = %+v", b)
	}
	if g := rows[2]; g.Organization != "Gamma" || g.Model != "" || len(g.Status) < 6 || g.Status[:6] != "Error:" {
		t.Errorf("failed org row = %+v", g)
	}
}
