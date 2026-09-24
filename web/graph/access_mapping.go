package graph

import (
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
)

func toGraphAccessStatus(status string) model.AccessStatus {
	switch status {
	case db.AccessPending:
		return model.AccessStatusPending
	case db.AccessApproved:
		return model.AccessStatusApproved
	case db.AccessRejected:
		return model.AccessStatusRejected
	case db.AccessRevoked:
		return model.AccessStatusRevoked
	case app.AccessNone:
		return model.AccessStatusNone
	}
	// An unknown value in the table must not read as approved.
	return model.AccessStatusNone
}

func toGraphAccess(u *db.UserAccess) *model.AccessEntry {
	return &model.AccessEntry{
		Email:       u.Email,
		Name:        u.Name,
		Department:  u.Department,
		Status:      toGraphAccessStatus(u.Status),
		Reason:      u.Reason,
		RequestedAt: u.RequestedAt,
		DecidedAt:   u.DecidedAt,
		DecidedBy:   u.DecidedBy,
	}
}

// toGraphAccessEntry takes a decision's result as is, so the resolvers stay one line.
func toGraphAccessEntry(u *db.UserAccess, err error) (*model.AccessEntry, error) {
	if err != nil {
		return nil, err
	}
	return toGraphAccess(u), nil
}
