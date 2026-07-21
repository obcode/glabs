package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// systemStateID is the fixed _id of the single system-state document. There is
// exactly one, holding server-wide bookkeeping that is not tied to any user.
const systemStateID = "state"

// SystemState holds server-wide bookkeeping. Today it records only when the last
// nightly summary was sent, so the digest window survives restarts and a summary
// is never sent twice for the same period.
type SystemState struct {
	ID            string     `bson:"_id"`
	SummarySentAt *time.Time `bson:"summarySentAt,omitempty"`
}

// SystemState returns the single state document, or a zero-valued one (never nil)
// if it does not exist yet.
func (db *DB) SystemState(ctx context.Context) (*SystemState, error) {
	var s SystemState
	err := db.collection(collectionSystem).FindOne(ctx, bson.M{"_id": systemStateID}).Decode(&s)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &SystemState{ID: systemStateID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read system state: %w", err)
	}
	return &s, nil
}

// SetSummarySentAt records when the nightly summary was last sent.
func (db *DB) SetSummarySentAt(ctx context.Context, at time.Time) error {
	_, err := db.collection(collectionSystem).UpdateByID(ctx, systemStateID,
		bson.M{"$set": bson.M{"summarySentAt": at}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("cannot record summary-sent time: %w", err)
	}
	return nil
}
