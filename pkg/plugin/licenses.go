package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dubsector/meraki-datasource/pkg/meraki"
)

// licenseRow is one organization's licensing, the same columns for both
// Meraki licensing models.
type licenseRow struct {
	Organization    string     `json:"organization"`
	OrganizationID  string     `json:"organizationId"`
	Model           string     `json:"model"`
	Status          string     `json:"status"`
	ExpirationDate  *time.Time `json:"expirationDate"`
	DaysLeft        *int       `json:"daysLeft"`
	LicensedDevices *int       `json:"licensedDevices"`
	Active          *int       `json:"active"`
	Expiring        *int       `json:"expiring"`
	Expired         *int       `json:"expired"`
	Unused          *int       `json:"unused"`
}

type orgRef struct {
	ID   orgID  `json:"id"`
	Name string `json:"name"`
}

// licenseRows reads every organization in orgs at once, a few at a time.
// An organization that fails, often one with API access turned off, gets
// a row saying why instead of failing the whole query.
func licenseRows(ctx context.Context, c *meraki.Client, orgs []orgRef, now time.Time) []json.RawMessage {
	rows := make([]licenseRow, len(orgs))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, o := range orgs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			raw, err := c.ForOrg(string(o.ID)).LicensesOverview(ctx)
			row := licenseRow{Organization: o.Name, OrganizationID: string(o.ID)}
			switch {
			case err != nil:
				row.Status = "Error: " + err.Error()
			case len(raw) > 0:
				row = parseLicenseOverview(raw[0], row, now)
			}
			rows[i] = row
		})
	}
	wg.Wait()
	// Soonest expiry first, so the table reads as a renewal list.
	slices.SortStableFunc(rows, func(a, b licenseRow) int {
		da, db := math.MaxInt, math.MaxInt
		if a.DaysLeft != nil {
			da = *a.DaysLeft
		}
		if b.DaysLeft != nil {
			db = *b.DaysLeft
		}
		return cmp.Or(cmp.Compare(da, db), strings.Compare(a.Organization, b.Organization))
	})
	return marshalRows(rows)
}

func parseLicenseOverview(raw json.RawMessage, row licenseRow, now time.Time) licenseRow {
	var o struct {
		Status         string         `json:"status"`
		ExpirationDate string         `json:"expirationDate"`
		DeviceCounts   map[string]int `json:"licensedDeviceCounts"`
		LicenseCount   *int           `json:"licenseCount"`
		States         struct {
			Active   struct{ Count int } `json:"active"`
			Expired  struct{ Count int } `json:"expired"`
			Expiring struct{ Count int } `json:"expiring"`
			Unused   struct{ Count int } `json:"unused"`
		} `json:"states"`
	}
	if json.Unmarshal(raw, &o) != nil {
		row.Status = "Unreadable license overview"
		return row
	}
	if o.LicenseCount != nil {
		s := o.States
		row.Model = "Per-device"
		row.Active, row.Expiring, row.Expired, row.Unused = &s.Active.Count, &s.Expiring.Count, &s.Expired.Count, &s.Unused.Count
		row.Status = "OK"
		switch {
		case s.Expired.Count > 0:
			row.Status = "Licenses expired"
		case s.Expiring.Count > 0:
			row.Status = "Licenses expiring"
		}
		return row
	}
	row.Model = "Co-termination"
	row.Status = o.Status
	total := 0
	for _, n := range o.DeviceCounts {
		total += n
	}
	row.LicensedDevices = &total
	// Meraki writes the date as "May 13, 2028 UTC".
	if t, err := time.Parse("Jan 2, 2006 MST", o.ExpirationDate); err == nil {
		days := int(math.Floor(t.Sub(now).Hours() / 24))
		row.ExpirationDate, row.DaysLeft = &t, &days
	}
	return row
}
