package storetest

import (
	"testing"
	"time"
)

func runSystem(t *testing.T, newStore NewStore) {
	t.Helper()

	t.Run("state before anything was written is a zero value, not nil", func(t *testing.T) {
		// The nightly digest asks for the state on every tick, including the very
		// first one after a fresh install. A nil here would panic the scheduler
		// on a server that has never sent a summary.
		s := newStore(t)
		got, err := s.SystemState(t.Context())
		if err != nil {
			t.Fatalf("SystemState: %v", err)
		}
		if got == nil {
			t.Fatal("SystemState = nil, want a zero-valued state")
		}
		if got.SummarySentAt != nil {
			t.Errorf("SummarySentAt = %v, want nil", got.SummarySentAt)
		}
	})

	t.Run("the summary timestamp survives a restart", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		if err := s.SetSummarySentAt(ctx, SummerInstant()); err != nil {
			t.Fatalf("SetSummarySentAt: %v", err)
		}
		got, err := s.SystemState(ctx)
		if err != nil {
			t.Fatalf("SystemState: %v", err)
		}
		if got.SummarySentAt == nil || !got.SummarySentAt.Equal(SummerInstant()) {
			t.Fatalf("SummarySentAt = %v, want %v", got.SummarySentAt, SummerInstant())
		}

		// Writing again moves it rather than adding a second record — the whole
		// point is that the digest window cannot be sent twice for one period.
		later := SummerInstant().Add(24 * time.Hour)
		if err := s.SetSummarySentAt(ctx, later); err != nil {
			t.Fatalf("SetSummarySentAt (second): %v", err)
		}
		got, err = s.SystemState(ctx)
		if err != nil {
			t.Fatalf("SystemState (second): %v", err)
		}
		if !got.SummarySentAt.Equal(later) {
			t.Errorf("SummarySentAt = %v, want %v", got.SummarySentAt, later)
		}
	})
}
