package main

import (
	"fmt"
	"time"

	"github.com/gastownhall/gascity/internal/clock"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/dispatch"
)

// drainReservationSweepInterval bounds how often the sweep scans closed drain
// controls. A closed drain never releases its members when it is closed outside
// gc (gascity#9179), so the scan only needs to be frequent enough that a stuck
// reservation clears well before the next drain wants its members.
const drainReservationSweepInterval = 5 * time.Minute

// runDrainReservationSweep clears member reservations that drains closed
// outside gc still hold (gascity#9179). It runs at most once per
// drainReservationSweepInterval over the city's graph and work stores, taken
// from the runtime's typed accessors so this file adds no store conversions.
// Rig-scoped stores are not swept; see the PR's known limits.
func (cr *CityRuntime) runDrainReservationSweep(_ *config.City, now time.Time) {
	if !cr.drainReservationSweepLast.IsZero() && now.Sub(cr.drainReservationSweepLast) < clock.Backstop(drainReservationSweepInterval) {
		return
	}
	cr.drainReservationSweepLast = now

	if cr.cityBeadStore() == nil {
		// No city stores are open (a runtime that has not started its stores).
		return
	}
	graph := cr.graphBeadStore()
	work := cr.cityWorkStore()
	opts := controlDispatchOptions(cr.cityPath, cr.cityPath, graph, work, false)
	n, err := dispatch.ReleaseClosedDrainReservations(graph, opts)
	if err != nil && cr.stderr != nil {
		fmt.Fprintf(cr.stderr, "%s: drain reservation sweep (swept %d): %v\n", cr.logPrefix, n, err) //nolint:errcheck // best-effort stderr
	}
}
