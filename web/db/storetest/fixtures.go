package storetest

import (
	"time"

	"github.com/obcode/glabs/v3/config"
)

func ptr[T any](v T) *T { return &v }

// SummerInstant and WinterInstant are the two timestamps the suite writes: one
// on each side of the DST boundary, +02:00 in summer and +01:00 in winter. A
// store that hands back UTC passes a test that only uses one of them, because
// the two are indistinguishable when the offset is never compared.
//
// FUNCTIONS, not variables, and that is the whole point. A package-level
// variable is initialised before TestMain runs, so it would capture whatever
// time.Local happened to be at process start -- Europe/Berlin in the dev
// container, which has TZ set, and UTC on a GitHub runner, which does not. The
// suite then compared a UTC instant against a correctly Berlin-zoned result and
// failed on the runner only. Reading time.Local at call time makes the fixtures
// agree with TestMain wherever they run.
func SummerInstant() time.Time { return time.Date(2026, 7, 15, 14, 30, 0, 0, time.Local) }
func WinterInstant() time.Time { return time.Date(2026, 1, 15, 14, 30, 0, 0, time.Local) }

// FullCourseSource is a course source with EVERY field of config.CourseSource
// and its sub-structs set to a non-zero value, including both states of each
// pointer field — set where absent and present differ, nil where that is the
// interesting case.
//
// It is the fixture for the round-trip test, whose whole point is that a field
// which is not stored shows up as a difference. A fixture that leaves fields at
// their zero value cannot do that: zero survives being dropped.
//
// It is hand-written and therefore goes stale the moment a field is added to
// config/source.go. The reflection-based check that catches exactly that is a
// separate test over the json tags; this one is here to prove the storage path
// carries the values, not just the names.
func FullCourseSource() *config.CourseSource {
	return &config.CourseSource{
		Name:                   "fopra",
		CoursePath:             "fk07/fopra",
		SemesterPath:           "2026ws",
		UseCoursenameAsPrefix:  true,
		UseEmailDomainAsSuffix: ptr(false), // absent would mean true — false is only expressible explicitly
		Students:               []string{"a@hm.edu", "b@hm.edu"},
		Groups:                 map[string][]string{"gruppe-1": {"a@hm.edu", "b@hm.edu"}},
		Assignments: map[string]*config.AssignmentSource{
			"blatt01": {
				Extends:           "base",
				Abstract:          false,
				AssignmentPath:    "blatt01",
				Description:       "Erstes Übungsblatt",
				Per:               "student",
				ContainerRegistry: true,
				AccessLevel:       "developer",
				Students:          []string{"c@hm.edu"},
				Groups:            map[string][]string{"gruppe-2": {"c@hm.edu"}},
				MergeRequest: &config.MergeRequestSource{
					MergeMethod:                   "ff",
					SquashOption:                  "always",
					Pipeline:                      true,
					SkippedPipelinesAreSuccessful: true,
					AllThreadsMustBeResolved:      true,
					StatusChecksMustSucceed:       true,
					Approvals: &config.ApprovalsSource{
						Settings: &config.ApprovalSettingsSource{
							PreventApprovalByMergeRequestCreator:       ptr(true),
							PreventApprovalsByUsersWhoAddCommits:       ptr(false),
							PreventEditingApprovalRulesInMergeRequests: ptr(true),
							RequireUserReauthenticationToApprove:       ptr(false),
							WhenCommitAdded:                            ptr("reset_approvals"),
						},
						Rules: []config.ApprovalRuleSource{{
							Name:                  "tutor",
							Branch:                "main",
							Branches:              []string{"main", "dev"},
							Usernames:             []string{"tutor1"},
							Groups:                []string{"fk07/tutoren"},
							MultiMemberGroupsOnly: true,
							RequiredApprovals:     2,
						}},
					},
				},
				Branches: []config.BranchRuleSource{{
					Name:                      "main",
					Protect:                   true,
					MergeOnly:                 true,
					Default:                   true,
					AllowForcePush:            true,
					CodeOwnerApprovalRequired: true,
				}},
				Issues: &config.IssuesSource{
					ReplicateFromStartercode: true,
					IssueNumbers:             []int{1, 2, 3},
					IncludeChildTasks:        true,
				},
				Startercode: &config.StartercodeSource{
					URL:                       "https://gitlab.example.org/fk07/starter.git",
					FromBranch:                "main",
					Tag:                       "v1.0.0",
					Template:                  true,
					TemplateMessage:           "Startercode",
					ToBranch:                  "main",
					AdditionalBranches:        []string{"dev"},
					ProtectToBranch:           true,
					ProtectDevBranchMergeOnly: true,
					IssueNumbers:              []int{4, 5},
					// The deprecated pair is here on purpose, and has to stay: courses
					// imported years ago still carry them, `glabs config migrate` is
					// opt-in, and a storage format that quietly dropped them would take
					// the old configs' meaning with it. Deprecated is not unused.
					DevBranch:      "dev", //nolint:staticcheck // deliberately covered: old configs still have it
					ReplicateIssue: true,  //nolint:staticcheck // deliberately covered: old configs still have it
				},
				DeferredBranches: map[string]*config.DeferredBranchSource{
					"loesung": {
						URL:           ptr("https://gitlab.example.org/fk07/loesung.git"),
						FromBranch:    "main",
						ToBranch:      ptr("loesung"),
						Orphan:        ptr(false), // absent would mean true
						OrphanMessage: ptr("Lösung"),
					},
					// A second entry with every pointer nil: "absent" has to survive
					// the round trip as absent, not as the zero value.
					"nachreichung": {FromBranch: "main"},
				},
				Clone: &config.CloneSource{
					LocalPath: ptr("./abgaben"),
					Branch:    ptr("main"),
					Force:     true,
				},
				Release: &config.ReleaseSource{
					MergeRequest: &config.ReleaseMergeRequestSource{
						Source:   "dev",
						Target:   "main",
						Pipeline: true,
					},
					DockerImages: []string{"golang:1.26"},
				},
				Seeder: &config.SeederSource{
					Cmd:             "make",
					Args:            []string{"seed"},
					Name:            "glabs",
					EMail:           "glabs@example.org",
					SignKey:         "ABCDEF",
					ToBranch:        "main",
					ProtectToBranch: true,
				},
			},
			// An abstract assignment that only carries the fields a base usually
			// carries — it also proves the assignment map holds more than one key.
			"base": {
				Abstract:    true,
				Per:         "group",
				AccessLevel: "maintainer",
			},
		},
	}
}
