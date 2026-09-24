package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/obcode/glabs/v3/web/principal"
)

func (f *fakeStore) GetUserAccess(_ context.Context, email string) (*db.UserAccess, error) {
	f.userReads++
	u := f.users[email]
	if u == nil {
		return nil, nil
	}
	c := *u
	return &c, nil
}

func (f *fakeStore) ListUserAccess(_ context.Context) ([]*db.UserAccess, error) {
	var out []*db.UserAccess
	for _, u := range f.users {
		c := *u
		out = append(out, &c)
	}
	return out, nil
}

func (f *fakeStore) InsertAccessRequest(_ context.Context, u *db.UserAccess) (bool, error) {
	if f.users == nil {
		f.users = map[string]*db.UserAccess{}
	}
	if _, ok := f.users[u.Email]; ok {
		return false, nil
	}
	c := *u
	f.users[u.Email] = &c
	return true, nil
}

func (f *fakeStore) SetUserAccessStatus(_ context.Context, email, status, decidedBy string, decidedAt time.Time) (*db.UserAccess, error) {
	u := f.users[email]
	if u == nil {
		return nil, nil
	}
	u.Status, u.DecidedBy, u.DecidedAt = status, decidedBy, &decidedAt
	c := *u
	return &c, nil
}

func (f *fakeStore) DeleteUserAccess(_ context.Context, email string) (bool, error) {
	_, ok := f.users[email]
	delete(f.users, email)
	return ok, nil
}

const (
	testAdmin  = "admin@hm.edu"
	testAdmin2 = "admin2@hm.edu"
	testUser   = "neu@hm.edu"
)

func newAccessApp() (*App, *fakeStore, *fakeMailer) {
	fs := newFakeStore()
	fm := &fakeMailer{}
	a := New(fs, nil, "", fm, false, []string{testAdmin, testAdmin2})
	a.SetPublicURL("https://glabs.example/")
	return a, fs, fm
}

func as(email string) context.Context {
	return principal.WithUser(context.Background(), &model.User{Email: email, Name: "Neue Person", Department: "07"})
}

func TestAccessStatus_adminsAlwaysApprovedWithoutRow(t *testing.T) {
	a, fs, _ := newAccessApp()
	status, err := a.AccessStatus(context.Background(), "ADMIN@hm.edu ")
	if err != nil || status != db.AccessApproved {
		t.Fatalf("admin status = %q, %v; want approved", status, err)
	}
	if fs.userReads != 0 {
		t.Errorf("an admin should not need a database read, got %d", fs.userReads)
	}
}

func TestAccessStatus_unknownIsNoneAndGateCloses(t *testing.T) {
	a, _, _ := newAccessApp()
	status, err := a.AccessStatus(context.Background(), testUser)
	if err != nil || status != AccessNone {
		t.Fatalf("status = %q, %v; want none", status, err)
	}
	if a.IsApproved(as(testUser)) {
		t.Error("a user who never asked must not be approved")
	}
	if a.IsApproved(context.Background()) {
		t.Error("no principal must not be approved")
	}
}

func TestAccessStatus_authDisabledApprovesEveryone(t *testing.T) {
	a, _, _ := newAccessApp()
	a.SetAuthDisabled(true)
	if !a.IsApproved(as(testUser)) {
		t.Error("with auth disabled the dev user must be approved")
	}
}

func TestAccessStatus_zeroAppIsClosed(t *testing.T) {
	// An App built without SetAuthDisabled must not let anyone in.
	a := New(newFakeStore(), nil, "", nil, false, nil)
	if a.IsApproved(as(testUser)) {
		t.Error("the zero value must keep the gate closed")
	}
}

func TestAccessStatus_storeErrorIsNotApproved(t *testing.T) {
	a := New(&failingAccessStore{newFakeStore()}, nil, "", nil, false, nil)
	if a.IsApproved(as(testUser)) {
		t.Error("a database error must close the gate")
	}
}

type failingAccessStore struct{ *fakeStore }

func (f *failingAccessStore) GetUserAccess(context.Context, string) (*db.UserAccess, error) {
	return nil, errors.New("db down")
}

func TestRequestAccess_storesMailsAdminsOnce(t *testing.T) {
	a, fs, fm := newAccessApp()
	ctx := as(testUser)

	if err := a.RequestAccess(ctx, "  Für Softwareentwicklung 2 `rm -rf` "); err != nil {
		t.Fatalf("RequestAccess: %v", err)
	}
	row := fs.users[testUser]
	if row == nil || row.Status != db.AccessPending || row.Name != "Neue Person" || row.Department != "07" {
		t.Fatalf("stored row = %+v", row)
	}
	if row.Reason != "Für Softwareentwicklung 2 `rm -rf`" {
		t.Errorf("reason = %q, want it trimmed and otherwise as written", row.Reason)
	}
	if len(fm.sent) != 2 {
		t.Fatalf("sent %d mails, want one per admin (2)", len(fm.sent))
	}
	for _, m := range fm.sent {
		if m.to != testAdmin && m.to != testAdmin2 {
			t.Errorf("mail to %q, want an admin", m.to)
		}
		if !strings.Contains(m.subject, testUser) {
			t.Errorf("subject %q should name the requester", m.subject)
		}
	}
	if len(fs.events) != 1 || fs.events[0].Type != db.EventAccessRequested {
		t.Errorf("events = %+v, want one access-requested", fs.events)
	}

	// Asking again while pending changes nothing and sends nothing.
	if err := a.RequestAccess(ctx, "nochmal"); err != nil {
		t.Fatalf("second RequestAccess: %v", err)
	}
	if len(fm.sent) != 2 || fs.users[testUser].Reason != row.Reason {
		t.Error("a repeated request must neither mail nor overwrite")
	}
}

func TestRequestAccess_refusedAfterDecision(t *testing.T) {
	a, _, _ := newAccessApp()
	if err := a.RequestAccess(as(testUser), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RejectUser(as(testAdmin), testUser); err != nil {
		t.Fatal(err)
	}
	if err := a.RequestAccess(as(testUser), ""); err == nil {
		t.Error("a rejected user must not be able to ask again until reset")
	}
	if _, err := a.ResetUser(as(testAdmin), testUser); err != nil {
		t.Fatal(err)
	}
	if err := a.RequestAccess(as(testUser), ""); err != nil {
		t.Errorf("after a reset asking again must work: %v", err)
	}
}

func TestRequestAccess_reasonTooLong(t *testing.T) {
	a, fs, _ := newAccessApp()
	if err := a.RequestAccess(as(testUser), strings.Repeat("x", maxReasonLen+1)); err == nil {
		t.Error("an overlong reason must be refused")
	}
	if len(fs.users) != 0 {
		t.Error("nothing may be stored for a refused request")
	}
}

func TestDecisions_requireAdmin(t *testing.T) {
	a, _, _ := newAccessApp()
	_ = a.RequestAccess(as(testUser), "")
	other := as("other@hm.edu")

	if _, err := a.ApproveUser(other, testUser); err == nil {
		t.Error("approve must be admin-only")
	}
	if _, err := a.ApproveUser(as(testUser), testUser); err == nil {
		t.Error("nobody may approve themselves")
	}
	if _, err := a.RejectUser(other, testUser); err == nil {
		t.Error("reject must be admin-only")
	}
	if _, err := a.RevokeUser(other, testUser); err == nil {
		t.Error("revoke must be admin-only")
	}
	if _, err := a.ResetUser(other, testUser); err == nil {
		t.Error("reset must be admin-only")
	}
	if _, err := a.UserAccessList(other); err == nil {
		t.Error("the list must be admin-only")
	}
}

func TestApprove_mailsUserAndOpensGateAtOnce(t *testing.T) {
	a, fs, fm := newAccessApp()
	user := as(testUser)
	_ = a.RequestAccess(user, "")
	if a.IsApproved(user) { // also fills the cache with "pending"
		t.Fatal("pending must not be approved")
	}
	fm.sent = nil

	row, err := a.ApproveUser(as(testAdmin), " NEU@hm.edu")
	if err != nil {
		t.Fatalf("ApproveUser: %v", err)
	}
	if row.Status != db.AccessApproved || row.DecidedBy != testAdmin || row.DecidedAt == nil {
		t.Errorf("row = %+v", row)
	}
	if len(fm.sent) != 1 || fm.sent[0].to != testUser || !strings.Contains(fm.sent[0].subject, "freigeschaltet") {
		t.Errorf("mails = %+v, want one to the user", fm.sent)
	}
	if !a.IsApproved(user) {
		t.Error("approval must take effect at once, not after the cache expires")
	}
	if last := fs.events[len(fs.events)-1]; last.Type != db.EventAccessGranted || last.Actor != testAdmin {
		t.Errorf("last event = %+v, want access-granted by the admin", last)
	}

	// Revoking closes the gate again, without a mail.
	fm.sent = nil
	if _, err := a.RevokeUser(as(testAdmin), testUser); err != nil {
		t.Fatal(err)
	}
	if a.IsApproved(user) {
		t.Error("a revoked user must be locked out at once")
	}
	if len(fm.sent) != 0 {
		t.Errorf("revoke sent %d mails, want none", len(fm.sent))
	}
}

func TestReject_mailsUser(t *testing.T) {
	a, _, fm := newAccessApp()
	_ = a.RequestAccess(as(testUser), "")
	fm.sent = nil
	if _, err := a.RejectUser(as(testAdmin), testUser); err != nil {
		t.Fatal(err)
	}
	if len(fm.sent) != 1 || fm.sent[0].to != testUser {
		t.Errorf("mails = %+v, want one to the user", fm.sent)
	}
}

func TestDecide_withoutRequestIsError(t *testing.T) {
	a, _, _ := newAccessApp()
	if _, err := a.ApproveUser(as(testAdmin), "nie@hm.edu"); err == nil {
		t.Error("approving someone who never asked must be an error")
	}
}

func TestAccessCache_avoidsRepeatedReads(t *testing.T) {
	a, fs, _ := newAccessApp()
	for range 5 {
		a.IsApproved(as(testUser))
	}
	if fs.userReads != 1 {
		t.Errorf("reads = %d, want 1 within the TTL", fs.userReads)
	}
}

func TestAccessMailData_defusesHTML(t *testing.T) {
	d := accessMailData(&db.UserAccess{
		Email: testUser, Name: "<script>x</script>", Reason: "```\n<b>hi</b>",
	}, "")
	if strings.ContainsAny(d.Name, "<>") {
		t.Errorf("name = %q, angle brackets must be gone", d.Name)
	}
	if strings.Contains(d.Reason, "`") {
		t.Errorf("reason = %q, backticks must be gone so it cannot leave the code block", d.Reason)
	}
}

func TestRequestAccess_linkInAdminMail(t *testing.T) {
	a, _, _ := newAccessApp()
	var got string
	a.mailer = mailerFunc(func(to, subject string, text []byte) { got = string(text) })
	_ = a.RequestAccess(as(testUser), "")
	if !strings.Contains(got, "https://glabs.example/admin/access?user=neu%40hm.edu") {
		t.Errorf("admin mail should carry the approval link, got:\n%s", got)
	}
}

type mailerFunc func(to, subject string, text []byte)

func (f mailerFunc) Send(_ bool, to, subject string, text, _ []byte) error {
	f(to, subject, text)
	return nil
}

func previewAs(email string) context.Context {
	return principal.WithUser(context.Background(), &model.User{Email: email, Preview: true})
}

// Preview mode lets an admin walk the whole flow with their own identity.
func TestPreview_adminWalksTheRequestFlow(t *testing.T) {
	a, fs, fm := newAccessApp()
	preview := previewAs(testAdmin)

	if a.IsAdmin(preview) {
		t.Error("an admin in preview mode must not be an admin")
	}
	if a.IsApproved(preview) {
		t.Fatal("an admin in preview mode without a row must not be approved")
	}
	if err := a.RequestAccess(preview, "Test"); err != nil {
		t.Fatalf("RequestAccess in preview: %v", err)
	}
	if fs.users[testAdmin] == nil || len(fm.sent) != 2 {
		t.Fatalf("row = %+v, mails = %d; want a row and a mail to each admin", fs.users[testAdmin], len(fm.sent))
	}
	if _, err := a.ApproveUser(preview, testAdmin); err == nil {
		t.Error("no admin rights in preview mode: approving must fail")
	}

	// Out of preview the admin approves their own request, and the preview sees it.
	if _, err := a.ApproveUser(as(testAdmin), testAdmin); err != nil {
		t.Fatalf("ApproveUser: %v", err)
	}
	if !a.IsApproved(preview) {
		t.Error("after approval the preview must be approved")
	}
	if _, err := a.RevokeUser(as(testAdmin), testAdmin); err != nil {
		t.Fatal(err)
	}
	if a.IsApproved(preview) {
		t.Error("after revoking the preview must be locked out")
	}
	// Outside preview a revoked row does not matter: admins are always in.
	if !a.IsApproved(as(testAdmin)) || !a.IsAdmin(as(testAdmin)) {
		t.Error("outside preview an admin must stay approved and admin")
	}
}

// The preview flag of one request never changes how another email is judged.
func TestPreview_onlyAffectsTheCaller(t *testing.T) {
	a, _, _ := newAccessApp()
	status, err := a.AccessStatus(previewAs(testUser), testAdmin)
	if err != nil || status != db.AccessApproved {
		t.Errorf("admin status looked up during someone else's preview = %q, %v; want approved", status, err)
	}
}
