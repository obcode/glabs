package app

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// zpaLookupConcurrency bounds how many ZPA lookups run at once, so enriching a
// 40-student roster is one burst of parallel calls rather than 40 sequential ones,
// without hammering ZPA.
const zpaLookupConcurrency = 8

// CourseStudent is one roster entry, enriched with ZPA details when they could be
// found. Found is false (and the detail fields empty) when ZPA is not configured
// or has no unambiguous match — the GUI then shows just the email.
type CourseStudent struct {
	Email     string
	Found     bool
	FirstName string
	LastName  string
	Gender    string
	Group     string
	Mtknr     string
}

// CourseStudents returns the course-level roster of one of the caller's courses,
// each email enriched with ZPA details (concurrently). A ZPA failure for one
// student is logged and leaves that entry un-enriched rather than failing the whole
// page. The result is sorted by last name, then email.
func (a *App) CourseStudents(ctx context.Context, course string) ([]*CourseStudent, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	var emails []string
	if stored.Source != nil {
		emails = stored.Source.Students
	}
	students := make([]*CourseStudent, len(emails))
	for i, e := range emails {
		students[i] = &CourseStudent{Email: e}
	}

	if a.zpa != nil {
		sem := make(chan struct{}, zpaLookupConcurrency)
		var wg sync.WaitGroup
		for _, cs := range students {
			wg.Add(1)
			go func(cs *CourseStudent) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				s, err := a.zpa.StudentByEmail(ctx, cs.Email)
				if err != nil {
					log.Warn().Err(err).Str("email", cs.Email).Msg("ZPA lookup failed")
					return
				}
				if s != nil {
					cs.Found = true
					cs.FirstName = s.FirstName
					cs.LastName = s.LastName
					cs.Gender = s.Gender
					cs.Group = s.Group
					cs.Mtknr = s.Mtknr
				}
			}(cs)
		}
		wg.Wait()
	}

	// Enriched students first (sorted by last name), then the un-enriched ones
	// (sorted by email), so a missing ZPA match sinks to the bottom instead of the
	// top.
	sort.SliceStable(students, func(i, j int) bool {
		si, sj := students[i], students[j]
		if si.Found != sj.Found {
			return si.Found
		}
		if li, lj := strings.ToLower(si.LastName), strings.ToLower(sj.LastName); li != lj {
			return li < lj
		}
		return strings.ToLower(si.Email) < strings.ToLower(sj.Email)
	})
	return students, nil
}
