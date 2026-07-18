package app

import "testing"

func TestOpGuard_serializesPerKey(t *testing.T) {
	g := newOpGuard()

	rel, ok := g.tryBegin("prof@hm.edu", "uc", "blatt1")
	if !ok {
		t.Fatal("first begin should succeed")
	}
	if _, ok := g.tryBegin("prof@hm.edu", "uc", "blatt1"); ok {
		t.Fatal("a second op on the same (owner,course,assignment) must be rejected")
	}

	// A different assignment, course, or owner is allowed concurrently.
	if rel2, ok := g.tryBegin("prof@hm.edu", "uc", "blatt2"); !ok {
		t.Error("a different assignment should be allowed")
	} else {
		rel2()
	}
	if rel3, ok := g.tryBegin("other@hm.edu", "uc", "blatt1"); !ok {
		t.Error("a different owner should be allowed")
	} else {
		rel3()
	}

	// After release the key frees up.
	rel()
	if rel4, ok := g.tryBegin("prof@hm.edu", "uc", "blatt1"); !ok {
		t.Fatal("after release the same key should succeed again")
	} else {
		rel4()
	}
}
