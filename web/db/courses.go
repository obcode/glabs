package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/obcode/glabs/v3/config"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StoredCourse is a course as saved by one user. Ownership is strict: a course
// belongs to the user who imported it, and no other user can see or touch it.
//
// RawYAML is kept verbatim alongside the parsed Source so a download can return
// exactly what was uploaded — comments and key order and all — as long as the
// course has not been edited through the web. Re-encoding Source would lose them.
type StoredCourse struct {
	Owner      string               `bson:"owner"`
	Name       string               `bson:"name"`
	Source     *config.CourseSource `bson:"source"`
	RawYAML    []byte               `bson:"rawYAML,omitempty"`
	ImportedAt time.Time            `bson:"importedAt"`
	UpdatedAt  time.Time            `bson:"updatedAt"`
}

// ErrCourseNotFound is returned when a course does not exist for the given owner.
// It deliberately does not distinguish "does not exist" from "belongs to someone
// else": to one user, another user's course simply is not there.
var ErrCourseNotFound = errors.New("course not found")

// EnsureCourseIndexes makes (owner, name) unique — a user has at most one course
// of a given name, and the pair is how every query is keyed.
func (db *DB) EnsureCourseIndexes(ctx context.Context) error {
	_, err := db.collection(collectionCourses).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("cannot create courses index: %w", err)
	}
	return nil
}

// Every method below takes an owner and filters on it. There is deliberately no
// method that reads a course by name alone: owner isolation is enforced by the
// shape of this API, not by remembering to add a filter at each call site.

// CoursesOf returns the courses owned by the given user, sorted by name.
func (db *DB) CoursesOf(ctx context.Context, owner string) ([]*StoredCourse, error) {
	cur, err := db.collection(collectionCourses).Find(ctx,
		bson.M{"owner": owner},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot list courses: %w", err)
	}
	var courses []*StoredCourse
	if err := cur.All(ctx, &courses); err != nil {
		return nil, fmt.Errorf("cannot decode courses: %w", err)
	}
	return courses, nil
}

// CourseOf returns one course owned by the given user, or ErrCourseNotFound.
func (db *DB) CourseOf(ctx context.Context, owner, name string) (*StoredCourse, error) {
	var course StoredCourse
	err := db.collection(collectionCourses).
		FindOne(ctx, bson.M{"owner": owner, "name": name}).
		Decode(&course)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("cannot read course %s: %w", name, err)
	}
	return &course, nil
}

// SaveCourse inserts or replaces a course for its owner. The owner and name on
// the document are the key; a document can never be written under a different
// owner than the one on it.
func (db *DB) SaveCourse(ctx context.Context, course *StoredCourse) error {
	if course.Owner == "" {
		return fmt.Errorf("refusing to save a course without an owner")
	}
	_, err := db.collection(collectionCourses).ReplaceOne(ctx,
		bson.M{"owner": course.Owner, "name": course.Name},
		course,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("cannot save course %s: %w", course.Name, err)
	}
	return nil
}

// DeleteCourse removes a course owned by the given user. Deleting a course that
// does not exist for that owner is ErrCourseNotFound, not a silent success — so
// a delete of another user's course reports "not found" rather than pretending
// it worked.
func (db *DB) DeleteCourse(ctx context.Context, owner, name string) error {
	res, err := db.collection(collectionCourses).DeleteOne(ctx, bson.M{"owner": owner, "name": name})
	if err != nil {
		return fmt.Errorf("cannot delete course %s: %w", name, err)
	}
	if res.DeletedCount == 0 {
		return ErrCourseNotFound
	}
	return nil
}
