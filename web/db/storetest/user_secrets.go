package storetest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/secrets"
)

// sealed builds a SealedValue that is structurally what the sealer produces — a
// key version, a 12-byte GCM nonce and a ciphertext — without running the
// crypto. This layer only stores the three fields; whether they decrypt is the
// sealer's contract, not the store's.
func sealed(marker byte) secrets.SealedValue {
	nonce := make([]byte, 12)
	ciphertext := make([]byte, 48)
	for i := range nonce {
		nonce[i] = marker
	}
	for i := range ciphertext {
		ciphertext[i] = marker ^ 0xff
	}
	return secrets.SealedValue{KeyVersion: 1, Nonce: nonce, Ciphertext: ciphertext}
}

func runUserSecrets(t *testing.T, newStore NewStore) {
	t.Helper()

	t.Run("no secrets yet is not an error", func(t *testing.T) {
		// (nil, nil), NOT a not-found error: every caller in web/app treats a nil
		// result as "no token configured" and would turn an error into a failed
		// request on a perfectly normal first visit.
		s := newStore(t)
		got, err := s.GetUserSecret(t.Context(), "a@hm.edu")
		if err != nil {
			t.Fatalf("GetUserSecret: err = %v, want nil", err)
		}
		if got != nil {
			t.Errorf("GetUserSecret = %+v, want nil", got)
		}
	})

	t.Run("save and read back the sealed token", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		want := sealed(0x11)
		if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", want, SummerInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken: %v", err)
		}

		got, err := s.GetUserSecret(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("GetUserSecret: %v", err)
		}
		if got == nil || got.GitLab == nil {
			t.Fatalf("GetUserSecret = %+v, want a stored GitLab token", got)
		}
		if diff := cmp.Diff(want, *got.GitLab); diff != "" {
			t.Errorf("sealed value did not survive the round trip (-want +got):\n%s", diff)
		}
		if got.Owner != "a@hm.edu" {
			t.Errorf("Owner = %q, want a@hm.edu", got.Owner)
		}
		if got.GitLabUpdatedAt == nil || !got.GitLabUpdatedAt.Equal(SummerInstant) {
			t.Errorf("GitLabUpdatedAt = %v, want %v", got.GitLabUpdatedAt, SummerInstant)
		}
	})

	t.Run("saving again replaces the token", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", sealed(0x11), SummerInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken: %v", err)
		}
		second := sealed(0x22)
		if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", second, WinterInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken (second): %v", err)
		}

		got, err := s.GetUserSecret(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("GetUserSecret: %v", err)
		}
		if diff := cmp.Diff(second, *got.GitLab); diff != "" {
			t.Errorf("token is not the second one (-want +got):\n%s", diff)
		}
		if !got.GitLabUpdatedAt.Equal(WinterInstant) {
			t.Errorf("GitLabUpdatedAt = %v, want %v", got.GitLabUpdatedAt, WinterInstant)
		}
	})

	t.Run("delete clears the token but keeps the record", func(t *testing.T) {
		// The record surviving is the point: it is the anchor for the other
		// per-user secrets this document is meant to grow, and deleting a GitLab
		// token must not take them with it. GetUserSecret therefore still returns
		// a non-nil UserSecret afterwards — with GitLab nil.
		s := newStore(t)
		ctx := t.Context()

		if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", sealed(0x11), SummerInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken: %v", err)
		}
		if err := s.DeleteUserGitLabToken(ctx, "a@hm.edu"); err != nil {
			t.Fatalf("DeleteUserGitLabToken: %v", err)
		}

		got, err := s.GetUserSecret(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("GetUserSecret: %v", err)
		}
		if got == nil {
			t.Fatal("GetUserSecret = nil, want the record with no GitLab token on it")
		}
		if got.GitLab != nil {
			t.Errorf("GitLab = %+v, want nil", got.GitLab)
		}
		if got.GitLabUpdatedAt != nil {
			t.Errorf("GitLabUpdatedAt = %v, want nil", got.GitLabUpdatedAt)
		}
	})

	t.Run("deleting a token nobody stored is a no-op", func(t *testing.T) {
		// Pinned as it is, not as it ought to be: the Mongo update matches no
		// document and reports success. The token-deletion path in web/app is
		// idempotent by design ("make sure there is none"), so this is the
		// behaviour above it relies on.
		s := newStore(t)
		if err := s.DeleteUserGitLabToken(t.Context(), "niemand@hm.edu"); err != nil {
			t.Errorf("DeleteUserGitLabToken for an unknown owner: err = %v, want nil", err)
		}
	})

	t.Run("secrets are per owner", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", sealed(0x11), SummerInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken a: %v", err)
		}
		if err := s.SaveUserGitLabToken(ctx, "b@hm.edu", sealed(0x22), SummerInstant); err != nil {
			t.Fatalf("SaveUserGitLabToken b: %v", err)
		}
		if err := s.DeleteUserGitLabToken(ctx, "a@hm.edu"); err != nil {
			t.Fatalf("DeleteUserGitLabToken a: %v", err)
		}

		got, err := s.GetUserSecret(ctx, "b@hm.edu")
		if err != nil {
			t.Fatalf("GetUserSecret b: %v", err)
		}
		if got == nil || got.GitLab == nil {
			t.Fatal("deleting one owner's token removed another owner's")
		}
		if diff := cmp.Diff(sealed(0x22), *got.GitLab); diff != "" {
			t.Errorf("b's token changed (-want +got):\n%s", diff)
		}
	})
}
