package app

import "sync"

// opGuard serializes mutating operations per (owner, course, assignment): a second
// operation on the SAME assignment is rejected while one runs, but operations on
// different assignments (or by different owners) run concurrently. Unlike plexams'
// single global mutex, the lock key is scoped so one prof's long op never blocks
// another's.
type opGuard struct {
	mu   sync.Mutex
	busy map[string]struct{}
}

func newOpGuard() *opGuard {
	return &opGuard{busy: make(map[string]struct{})}
}

// tryBegin reserves the (owner, course, assignment) slot. It returns a release
// function and true on success, or (nil, false) if an operation is already running
// for that key. The caller must call release exactly once when the op finishes.
func (g *opGuard) tryBegin(owner, course, assignment string) (release func(), ok bool) {
	key := owner + "\x00" + course + "\x00" + assignment
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, running := g.busy[key]; running {
		return nil, false
	}
	g.busy[key] = struct{}{}
	return func() {
		g.mu.Lock()
		delete(g.busy, key)
		g.mu.Unlock()
	}, true
}
