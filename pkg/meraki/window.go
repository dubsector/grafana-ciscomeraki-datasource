package meraki

import (
	"errors"
	"time"
)

const day = 24 * time.Hour

// Window is the time range an endpoint accepts: how far back t0 may reach and
// the longest t0..t1 span. Values come from the Meraki OpenAPI spec.
type Window struct {
	Lookback time.Duration
	MaxSpan  time.Duration
}

var (
	AvailabilityHistoryWindow = Window{Lookback: 31 * day, MaxSpan: 31 * day}
	SecurityEventsWindow      = Window{Lookback: 365 * day, MaxSpan: 365 * day}
	WirelessStatsWindow       = Window{Lookback: 180 * day, MaxSpan: 7 * day}
	ClientCountWindow         = Window{Lookback: 31 * day, MaxSpan: 31 * day}
	VPNStatsWindow            = Window{Lookback: 31 * day, MaxSpan: 31 * day}
	SensorHistoryWindow       = Window{Lookback: 365 * day, MaxSpan: 7 * day}
)

func (w Window) LookbackDays() int { return int(w.Lookback / day) }
func (w Window) MaxSpanDays() int  { return int(w.MaxSpan / day) }

// lookbackMargin keeps t0 inside the lookback limit by the time Meraki checks it.
const lookbackMargin = time.Minute

var ErrOutsideWindow = errors.New("time range is older than the Meraki API keeps for this query")

// Clamp fits [t0, t1] into w, keeping the most recent part of the range.
// changed reports whether t0 moved.
func (w Window) Clamp(t0, t1, now time.Time) (from, to time.Time, changed bool, err error) {
	from, to = t0, t1
	if earliest := now.Add(-w.Lookback + lookbackMargin); from.Before(earliest) {
		from = earliest
	}
	if to.Sub(from) > w.MaxSpan {
		from = to.Add(-w.MaxSpan)
	}
	if !to.After(from) {
		return t0, t1, false, ErrOutsideWindow
	}
	return from, to, !from.Equal(t0), nil
}
