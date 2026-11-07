package meraki

import (
	"errors"
	"testing"
	"time"
)

func TestWindowClamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	tests := []struct {
		name        string
		w           Window
		t0, t1      time.Time
		wantFrom    time.Time
		wantChanged bool
		wantErr     error
	}{
		{"inside limits", WirelessStatsWindow, ago(day), now, ago(day), false, nil},
		{"exactly max span", WirelessStatsWindow, ago(7 * day), now, ago(7 * day), false, nil},
		{"span too long keeps the latest part", WirelessStatsWindow, ago(30 * day), now, ago(7 * day), true, nil},
		{"lookback trims t0", ClientCountWindow, ago(40 * day), ago(10 * day), ago(31*day - lookbackMargin), true, nil},
		{"lookback and span together", VPNStatsWindow, ago(90 * day), now, ago(31*day - lookbackMargin), true, nil},
		{"range entirely too old", ClientCountWindow, ago(60 * day), ago(40 * day), time.Time{}, false, ErrOutsideWindow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, changed, err := tt.w.Clamp(tt.t0, tt.t1, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !from.Equal(tt.wantFrom) || !to.Equal(tt.t1) || changed != tt.wantChanged {
				t.Errorf("got %s..%s changed=%v, want %s..%s changed=%v", from, to, changed, tt.wantFrom, tt.t1, tt.wantChanged)
			}
		})
	}
}
