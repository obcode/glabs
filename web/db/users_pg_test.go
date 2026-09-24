package db_test

import (
	"testing"
	"time"

	"github.com/obcode/glabs/v3/internal/pgtest"
	"github.com/obcode/glabs/v3/web/db"
)

// The users table is PostgreSQL only, so it is tested here rather than in the
// store contract suite that also runs against MongoDB.
func TestPGUserAccess(t *testing.T) {
	pg := pgtest.NewDB(t)
	ctx := t.Context()
	requestedAt := func() time.Time { return time.Date(2026, 9, 24, 10, 0, 0, 0, time.Local) }

	if u, err := pg.GetUserAccess(ctx, "neu@hm.edu"); err != nil || u != nil {
		t.Fatalf("unknown user = %+v, %v; want nil, nil", u, err)
	}

	req := &db.UserAccess{Email: "neu@hm.edu", Name: "Neu", Department: "07", Reason: "SE2", RequestedAt: requestedAt()}
	created, err := pg.InsertAccessRequest(ctx, req)
	if err != nil || !created {
		t.Fatalf("first insert = %v, %v; want created", created, err)
	}
	// A second request must neither fail nor overwrite the first.
	created, err = pg.InsertAccessRequest(ctx, &db.UserAccess{Email: "neu@hm.edu", Reason: "anders", RequestedAt: time.Now()})
	if err != nil || created {
		t.Fatalf("second insert = %v, %v; want not created", created, err)
	}

	got, err := pg.GetUserAccess(ctx, "neu@hm.edu")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != db.AccessPending || got.Reason != "SE2" || got.Department != "07" ||
		!got.RequestedAt.Equal(requestedAt()) || got.DecidedAt != nil {
		t.Errorf("stored = %+v", got)
	}

	// A decided row and a second pending one: the pending one lists first.
	if _, err := pg.InsertAccessRequest(ctx, &db.UserAccess{Email: "zwei@hm.edu", RequestedAt: requestedAt().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	decidedAt := requestedAt().Add(time.Hour)
	row, err := pg.SetUserAccessStatus(ctx, "neu@hm.edu", db.AccessApproved, "admin@hm.edu", decidedAt)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != db.AccessApproved || row.DecidedBy != "admin@hm.edu" || row.DecidedAt == nil || !row.DecidedAt.Equal(decidedAt) {
		t.Errorf("decided row = %+v", row)
	}
	list, err := pg.ListUserAccess(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Email != "zwei@hm.edu" {
		t.Errorf("list order = %v, want the pending request first", emails(list))
	}

	if row, err := pg.SetUserAccessStatus(ctx, "nie@hm.edu", db.AccessApproved, "admin@hm.edu", decidedAt); err != nil || row != nil {
		t.Errorf("deciding about a missing row = %+v, %v; want nil, nil", row, err)
	}
	if _, err := pg.SetUserAccessStatus(ctx, "neu@hm.edu", "admin", "admin@hm.edu", decidedAt); err == nil {
		t.Error("an unknown status must be refused by the check constraint")
	}

	if existed, err := pg.DeleteUserAccess(ctx, "neu@hm.edu"); err != nil || !existed {
		t.Errorf("delete = %v, %v; want existed", existed, err)
	}
	if existed, err := pg.DeleteUserAccess(ctx, "neu@hm.edu"); err != nil || existed {
		t.Errorf("second delete = %v, %v; want not existed", existed, err)
	}
}

func emails(us []*db.UserAccess) []string {
	out := make([]string, 0, len(us))
	for _, u := range us {
		out = append(out, u.Email)
	}
	return out
}
