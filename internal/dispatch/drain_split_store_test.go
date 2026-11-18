package dispatch

import (
	"testing"

	"github.com/gastownhall/gascity/internal/beadmeta"
	"github.com/gastownhall/gascity/internal/beads"
)

// Known limit for gascity#9179 (documented in the PR): a drain control in
// store A whose members live in store B. Neither the sweep nor the owner check
// in #9202 can see across stores, so the reservation survives. These tests pin
// that limit so a future change to it is deliberate.
func TestSplitStoreSweepDoesNotReachMemberInOtherStore(t *testing.T) {
	storeA := beads.NewMemStore()
	storeB := beads.NewMemStore()

	member, err := storeB.Create(beads.Bead{Title: "member", Type: "task"})
	if err != nil {
		t.Fatalf("Create(member): %v", err)
	}
	drain, err := storeA.Create(beads.Bead{Title: "drain", Type: "control", Metadata: map[string]string{
		beadmeta.KindMetadataKey:          beadmeta.KindDrain,
		beadmeta.DrainManifestMetadataKey: `{"version":1,"rows":[{"member_id":"` + member.ID + `"}]}`,
	}})
	if err != nil {
		t.Fatalf("Create(drain): %v", err)
	}
	if err := storeA.Close(drain.ID); err != nil {
		t.Fatalf("Close(drain): %v", err)
	}
	if err := storeB.SetMetadata(member.ID, beadmeta.ExclusiveDrainReservationMetadataKey, drain.ID); err != nil {
		t.Fatalf("SetMetadata(reservation): %v", err)
	}

	opts := ProcessOptions{Work: beads.WorkStore{Store: storeA}}
	if _, err := ReleaseClosedDrainReservations(beads.GraphStore{Store: storeA}, opts); err != nil {
		t.Fatalf("ReleaseClosedDrainReservations: %v", err)
	}
	got, err := storeB.Get(member.ID)
	if err != nil {
		t.Fatalf("Get(member): %v", err)
	}
	if got.Metadata[beadmeta.ExclusiveDrainReservationMetadataKey] != drain.ID {
		t.Fatalf("sweep reached the member in store B; the known limit no longer holds")
	}
}

func TestOwnerCheckIsStoreLocal(t *testing.T) {
	storeA := beads.NewMemStore()
	storeB := beads.NewMemStore()
	drain, err := storeA.Create(beads.Bead{Title: "drain", Type: "control"})
	if err != nil {
		t.Fatalf("Create(drain): %v", err)
	}
	if err := storeA.Close(drain.ID); err != nil {
		t.Fatalf("Close(drain): %v", err)
	}
	// The closed owner is visible in its own store...
	if closed, err := drainControlClosed(beads.GraphStore{Store: storeA}, drain.ID); err != nil || !closed {
		t.Fatalf("owner in its own store: closed=%v err=%v, want closed", closed, err)
	}
	// ...but a drain reserving from store B cannot see it, so #9202 treats it as live.
	if closed, err := drainControlClosed(beads.GraphStore{Store: storeB}, drain.ID); err != nil || closed {
		t.Fatalf("owner seen from store B: closed=%v err=%v, want not visible (limit)", closed, err)
	}
}
