package config

// Source form of the configuration: a faithful, typed 1:1 model of a course
// YAML file, before any resolution.
//
// This is deliberately NOT AssignmentConfig. AssignmentConfig is the *resolved*
// form — `extends` applied, course-level students merged in, Path/URL computed.
// The source form is what a human writes and what round-trips:
//
//	YAML  <->  CourseSource  <->  BSON
//	                  |
//	                  v  Resolve()
//	           AssignmentConfig
//
// Four tag sets, one per direction:
//   - yaml:        encoding back to a course file (config/encode.go)
//   - bson:        storage in MongoDB
//   - json:        storage in PostgreSQL, as the jsonb `source` column
//   - mapstructure: decoding (config/decode.go)
//
// bson and json are kept deliberately identical, name for name, so the same
// document reads the same out of either store and the migration between them is
// a copy rather than a translation. TestSourceTagsAreComplete enforces both the
// presence and the agreement; do not let them drift.
//
// Both are STORAGE FORMATS, which has a consequence worth stating plainly:
// renaming a field's tag does not rename anything in a database that already
// holds rows. It makes the old value unreadable, and the field comes back as its
// zero value with nothing reporting an error. Adding a field is safe; renaming
// or removing one is a data migration.
//
// Decoding goes through mapstructure rather than yaml.v3 directly because
// viper — the loader being replaced — matches keys case-insensitively, and
// real configs rely on it (`frombranch` and `fromBranch` both work today).
// yaml.v3 matches tags exactly and would silently drop such keys. mapstructure
// folds case by default, which is exactly viper's own behaviour: viper decodes
// with mapstructure internally.
//
// Pointer fields mark "absent" where absent and zero mean different things —
// i.e. exactly where the viper-based loader calls IsSet or checks a map lookup's
// ok. Everywhere else a plain value is enough, because the loader treats empty
// as absent anyway (`if len(x) > 0`).

// CourseSource is one course file. The file has a single top-level key (the
// course name) whose value is this struct.
type CourseSource struct {
	// Name is the file's top-level key. It is not part of the mapping itself.
	Name string `yaml:"-" bson:"name" json:"name" mapstructure:"-"`

	CoursePath   string `yaml:"coursepath,omitempty" bson:"coursepath,omitempty" json:"coursepath,omitempty" mapstructure:"coursepath"`
	SemesterPath string `yaml:"semesterpath,omitempty" bson:"semesterpath,omitempty" json:"semesterpath,omitempty" mapstructure:"semesterpath"`

	UseCoursenameAsPrefix bool `yaml:"useCoursenameAsPrefix,omitempty" bson:"useCoursenameAsPrefix,omitempty" json:"useCoursenameAsPrefix,omitempty" mapstructure:"useCoursenameAsPrefix"`
	// Pointer: absent means true (config/assignment.go:100-105), false means false.
	UseEmailDomainAsSuffix *bool `yaml:"useEmailDomainAsSuffix,omitempty" bson:"useEmailDomainAsSuffix,omitempty" json:"useEmailDomainAsSuffix,omitempty" mapstructure:"useEmailDomainAsSuffix"`

	Students []string            `yaml:"students,omitempty" bson:"students,omitempty" json:"students,omitempty" mapstructure:"students"`
	Groups   map[string][]string `yaml:"groups,omitempty" bson:"groups,omitempty" json:"groups,omitempty" mapstructure:"groups"`

	// Assignments are siblings of the course settings above, not nested under a
	// key. yaml ",inline" reproduces that layout on encode; mapstructure
	// ",remain" collects them on decode. In BSON they get their own subdocument
	// so they cannot collide with the course settings.
	Assignments map[string]*AssignmentSource `yaml:",inline" bson:"assignments,omitempty" json:"assignments,omitempty" mapstructure:",remain"`
}

// AssignmentSource is a single assignment as written in the course file, with
// `extends` unresolved.
type AssignmentSource struct {
	// Meta keys. Never inherited, never part of the effective config.
	Extends  string `yaml:"extends,omitempty" bson:"extends,omitempty" json:"extends,omitempty" mapstructure:"extends"`
	Abstract bool   `yaml:"abstract,omitempty" bson:"abstract,omitempty" json:"abstract,omitempty" mapstructure:"abstract"`

	AssignmentPath    string `yaml:"assignmentpath,omitempty" bson:"assignmentpath,omitempty" json:"assignmentpath,omitempty" mapstructure:"assignmentpath"`
	Description       string `yaml:"description,omitempty" bson:"description,omitempty" json:"description,omitempty" mapstructure:"description"`
	Per               string `yaml:"per,omitempty" bson:"per,omitempty" json:"per,omitempty" mapstructure:"per"`
	ContainerRegistry bool   `yaml:"containerRegistry,omitempty" bson:"containerRegistry,omitempty" json:"containerRegistry,omitempty" mapstructure:"containerRegistry"`
	AccessLevel       string `yaml:"accesslevel,omitempty" bson:"accesslevel,omitempty" json:"accesslevel,omitempty" mapstructure:"accesslevel"`

	Students []string            `yaml:"students,omitempty" bson:"students,omitempty" json:"students,omitempty" mapstructure:"students"`
	Groups   map[string][]string `yaml:"groups,omitempty" bson:"groups,omitempty" json:"groups,omitempty" mapstructure:"groups"`

	MergeRequest     *MergeRequestSource              `yaml:"mergeRequest,omitempty" bson:"mergeRequest,omitempty" json:"mergeRequest,omitempty" mapstructure:"mergeRequest"`
	Branches         []BranchRuleSource               `yaml:"branches,omitempty" bson:"branches,omitempty" json:"branches,omitempty" mapstructure:"branches"`
	Issues           *IssuesSource                    `yaml:"issues,omitempty" bson:"issues,omitempty" json:"issues,omitempty" mapstructure:"issues"`
	Startercode      *StartercodeSource               `yaml:"startercode,omitempty" bson:"startercode,omitempty" json:"startercode,omitempty" mapstructure:"startercode"`
	DeferredBranches map[string]*DeferredBranchSource `yaml:"deferredBranches,omitempty" bson:"deferredBranches,omitempty" json:"deferredBranches,omitempty" mapstructure:"deferredBranches"`
	Clone            *CloneSource                     `yaml:"clone,omitempty" bson:"clone,omitempty" json:"clone,omitempty" mapstructure:"clone"`
	Release          *ReleaseSource                   `yaml:"release,omitempty" bson:"release,omitempty" json:"release,omitempty" mapstructure:"release"`
	Seeder           *SeederSource                    `yaml:"seeder,omitempty" bson:"seeder,omitempty" json:"seeder,omitempty" mapstructure:"seeder"`
}

type StartercodeSource struct {
	URL                string   `yaml:"url,omitempty" bson:"url,omitempty" json:"url,omitempty" mapstructure:"url"`
	FromBranch         string   `yaml:"fromBranch,omitempty" bson:"fromBranch,omitempty" json:"fromBranch,omitempty" mapstructure:"fromBranch"`
	Tag                string   `yaml:"tag,omitempty" bson:"tag,omitempty" json:"tag,omitempty" mapstructure:"tag"`
	Template           bool     `yaml:"template,omitempty" bson:"template,omitempty" json:"template,omitempty" mapstructure:"template"`
	TemplateMessage    string   `yaml:"templateMessage,omitempty" bson:"templateMessage,omitempty" json:"templateMessage,omitempty" mapstructure:"templateMessage"`
	ToBranch           string   `yaml:"toBranch,omitempty" bson:"toBranch,omitempty" json:"toBranch,omitempty" mapstructure:"toBranch"`
	AdditionalBranches []string `yaml:"additionalBranches,omitempty" bson:"additionalBranches,omitempty" json:"additionalBranches,omitempty" mapstructure:"additionalBranches"`

	// Deprecated: superseded by the assignment-level `branches:` block, which
	// wins outright when present (config/repo.go:98). Read only for older
	// configs; `glabs config migrate` rewrites them.
	DevBranch                 string `yaml:"devBranch,omitempty" bson:"devBranch,omitempty" json:"devBranch,omitempty" mapstructure:"devBranch"`
	ProtectToBranch           bool   `yaml:"protectToBranch,omitempty" bson:"protectToBranch,omitempty" json:"protectToBranch,omitempty" mapstructure:"protectToBranch"`
	ProtectDevBranchMergeOnly bool   `yaml:"protectDevBranchMergeOnly,omitempty" bson:"protectDevBranchMergeOnly,omitempty" json:"protectDevBranchMergeOnly,omitempty" mapstructure:"protectDevBranchMergeOnly"`

	// Deprecated: superseded by the assignment-level `issues:` block, which
	// disables these entirely by merely being present (config/repo.go:193).
	ReplicateIssue bool  `yaml:"replicateIssue,omitempty" bson:"replicateIssue,omitempty" json:"replicateIssue,omitempty" mapstructure:"replicateIssue"`
	IssueNumbers   []int `yaml:"issueNumbers,omitempty" bson:"issueNumbers,omitempty" json:"issueNumbers,omitempty" mapstructure:"issueNumbers"`
}

type BranchRuleSource struct {
	Name                      string `yaml:"name,omitempty" bson:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	Protect                   bool   `yaml:"protect,omitempty" bson:"protect,omitempty" json:"protect,omitempty" mapstructure:"protect"`
	MergeOnly                 bool   `yaml:"mergeOnly,omitempty" bson:"mergeOnly,omitempty" json:"mergeOnly,omitempty" mapstructure:"mergeOnly"`
	Default                   bool   `yaml:"default,omitempty" bson:"default,omitempty" json:"default,omitempty" mapstructure:"default"`
	AllowForcePush            bool   `yaml:"allowForcePush,omitempty" bson:"allowForcePush,omitempty" json:"allowForcePush,omitempty" mapstructure:"allowForcePush"`
	CodeOwnerApprovalRequired bool   `yaml:"codeOwnerApprovalRequired,omitempty" bson:"codeOwnerApprovalRequired,omitempty" json:"codeOwnerApprovalRequired,omitempty" mapstructure:"codeOwnerApprovalRequired"`
}

type IssuesSource struct {
	ReplicateFromStartercode bool  `yaml:"replicateFromStartercode,omitempty" bson:"replicateFromStartercode,omitempty" json:"replicateFromStartercode,omitempty" mapstructure:"replicateFromStartercode"`
	IssueNumbers             []int `yaml:"issueNumbers,omitempty" bson:"issueNumbers,omitempty" json:"issueNumbers,omitempty" mapstructure:"issueNumbers"`
	IncludeChildTasks        bool  `yaml:"includeChildTasks,omitempty" bson:"includeChildTasks,omitempty" json:"includeChildTasks,omitempty" mapstructure:"includeChildTasks"`
}

type DeferredBranchSource struct {
	// Pointer: absent falls back to the startercode URL, empty does not
	// (config/assignment.go:66-68).
	URL        *string `yaml:"url,omitempty" bson:"url,omitempty" json:"url,omitempty" mapstructure:"url"`
	FromBranch string  `yaml:"fromBranch,omitempty" bson:"fromBranch,omitempty" json:"fromBranch,omitempty" mapstructure:"fromBranch"`
	// Pointer: absent falls back to FromBranch (config/assignment.go:71-74).
	ToBranch *string `yaml:"toBranch,omitempty" bson:"toBranch,omitempty" json:"toBranch,omitempty" mapstructure:"toBranch"`
	// Pointer: absent means true (config/assignment.go:75-79).
	Orphan *bool `yaml:"orphan,omitempty" bson:"orphan,omitempty" json:"orphan,omitempty" mapstructure:"orphan"`
	// Pointer: absent gets a generated message (config/assignment.go:81-84).
	OrphanMessage *string `yaml:"orphanMessage,omitempty" bson:"orphanMessage,omitempty" json:"orphanMessage,omitempty" mapstructure:"orphanMessage"`
}

type CloneSource struct {
	// Pointer: absent means "." (config/repo.go:212-215).
	LocalPath *string `yaml:"localpath,omitempty" bson:"localpath,omitempty" json:"localpath,omitempty" mapstructure:"localpath"`
	// Pointer: absent means the assignment's default branch (config/repo.go:217-220).
	Branch *string `yaml:"branch,omitempty" bson:"branch,omitempty" json:"branch,omitempty" mapstructure:"branch"`
	Force  bool    `yaml:"force,omitempty" bson:"force,omitempty" json:"force,omitempty" mapstructure:"force"`
}

type ReleaseSource struct {
	MergeRequest *ReleaseMergeRequestSource `yaml:"mergeRequest,omitempty" bson:"mergeRequest,omitempty" json:"mergeRequest,omitempty" mapstructure:"mergeRequest"`
	DockerImages []string                   `yaml:"dockerImages,omitempty" bson:"dockerImages,omitempty" json:"dockerImages,omitempty" mapstructure:"dockerImages"`
}

type ReleaseMergeRequestSource struct {
	Source   string `yaml:"source,omitempty" bson:"source,omitempty" json:"source,omitempty" mapstructure:"source"`
	Target   string `yaml:"target,omitempty" bson:"target,omitempty" json:"target,omitempty" mapstructure:"target"`
	Pipeline bool   `yaml:"pipeline,omitempty" bson:"pipeline,omitempty" json:"pipeline,omitempty" mapstructure:"pipeline"`
}

type SeederSource struct {
	Cmd             string   `yaml:"cmd,omitempty" bson:"cmd,omitempty" json:"cmd,omitempty" mapstructure:"cmd"`
	Args            []string `yaml:"args,omitempty" bson:"args,omitempty" json:"args,omitempty" mapstructure:"args"`
	Name            string   `yaml:"name,omitempty" bson:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	EMail           string   `yaml:"email,omitempty" bson:"email,omitempty" json:"email,omitempty" mapstructure:"email"`
	SignKey         string   `yaml:"signKey,omitempty" bson:"signKey,omitempty" json:"signKey,omitempty" mapstructure:"signKey"`
	ToBranch        string   `yaml:"toBranch,omitempty" bson:"toBranch,omitempty" json:"toBranch,omitempty" mapstructure:"toBranch"`
	ProtectToBranch bool     `yaml:"protectToBranch,omitempty" bson:"protectToBranch,omitempty" json:"protectToBranch,omitempty" mapstructure:"protectToBranch"`
}

type MergeRequestSource struct {
	MergeMethod                   string           `yaml:"mergeMethod,omitempty" bson:"mergeMethod,omitempty" json:"mergeMethod,omitempty" mapstructure:"mergeMethod"`
	SquashOption                  string           `yaml:"squashOption,omitempty" bson:"squashOption,omitempty" json:"squashOption,omitempty" mapstructure:"squashOption"`
	Pipeline                      bool             `yaml:"pipeline,omitempty" bson:"pipeline,omitempty" json:"pipeline,omitempty" mapstructure:"pipeline"`
	SkippedPipelinesAreSuccessful bool             `yaml:"skippedPipelinesAreSuccessful,omitempty" bson:"skippedPipelinesAreSuccessful,omitempty" json:"skippedPipelinesAreSuccessful,omitempty" mapstructure:"skippedPipelinesAreSuccessful"`
	AllThreadsMustBeResolved      bool             `yaml:"allThreadsMustBeResolved,omitempty" bson:"allThreadsMustBeResolved,omitempty" json:"allThreadsMustBeResolved,omitempty" mapstructure:"allThreadsMustBeResolved"`
	StatusChecksMustSucceed       bool             `yaml:"statusChecksMustSucceed,omitempty" bson:"statusChecksMustSucceed,omitempty" json:"statusChecksMustSucceed,omitempty" mapstructure:"statusChecksMustSucceed"`
	Approvals                     *ApprovalsSource `yaml:"approvals,omitempty" bson:"approvals,omitempty" json:"approvals,omitempty" mapstructure:"approvals"`
}

// ApprovalsSource is polymorphic in the source: either a bare list of rules
// (the original form) or a {settings, rules} mapping. Decoding normalizes the
// list form into Rules (see approvalsDecodeHook); encoding always emits the
// mapping form, which is how `glabs config migrate` upgrades old files.
type ApprovalsSource struct {
	Settings *ApprovalSettingsSource `yaml:"settings,omitempty" bson:"settings,omitempty" json:"settings,omitempty" mapstructure:"settings"`
	Rules    []ApprovalRuleSource    `yaml:"rules,omitempty" bson:"rules,omitempty" json:"rules,omitempty" mapstructure:"rules"`
}

type ApprovalRuleSource struct {
	Name string `yaml:"name,omitempty" bson:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	// Deprecated: singular form, folded into Branches on resolve.
	Branch                string   `yaml:"branch,omitempty" bson:"branch,omitempty" json:"branch,omitempty" mapstructure:"branch"`
	Branches              []string `yaml:"branches,omitempty" bson:"branches,omitempty" json:"branches,omitempty" mapstructure:"branches"`
	Usernames             []string `yaml:"usernames,omitempty" bson:"usernames,omitempty" json:"usernames,omitempty" mapstructure:"usernames"`
	Groups                []string `yaml:"groups,omitempty" bson:"groups,omitempty" json:"groups,omitempty" mapstructure:"groups"`
	MultiMemberGroupsOnly bool     `yaml:"multiMemberGroupsOnly,omitempty" bson:"multiMemberGroupsOnly,omitempty" json:"multiMemberGroupsOnly,omitempty" mapstructure:"multiMemberGroupsOnly"`
	RequiredApprovals     int      `yaml:"requiredApprovals,omitempty" bson:"requiredApprovals,omitempty" json:"requiredApprovals,omitempty" mapstructure:"requiredApprovals"`
}

// ApprovalSettingsSource fields are pointers throughout: each is only sent to
// GitLab when explicitly configured (config/assignment.go:385-412).
type ApprovalSettingsSource struct {
	PreventApprovalByMergeRequestCreator       *bool   `yaml:"preventApprovalByMergeRequestCreator,omitempty" bson:"preventApprovalByMergeRequestCreator,omitempty" json:"preventApprovalByMergeRequestCreator,omitempty" mapstructure:"preventApprovalByMergeRequestCreator"`
	PreventApprovalsByUsersWhoAddCommits       *bool   `yaml:"preventApprovalsByUsersWhoAddCommits,omitempty" bson:"preventApprovalsByUsersWhoAddCommits,omitempty" json:"preventApprovalsByUsersWhoAddCommits,omitempty" mapstructure:"preventApprovalsByUsersWhoAddCommits"`
	PreventEditingApprovalRulesInMergeRequests *bool   `yaml:"preventEditingApprovalRulesInMergeRequests,omitempty" bson:"preventEditingApprovalRulesInMergeRequests,omitempty" json:"preventEditingApprovalRulesInMergeRequests,omitempty" mapstructure:"preventEditingApprovalRulesInMergeRequests"`
	RequireUserReauthenticationToApprove       *bool   `yaml:"requireUserReauthenticationToApprove,omitempty" bson:"requireUserReauthenticationToApprove,omitempty" json:"requireUserReauthenticationToApprove,omitempty" mapstructure:"requireUserReauthenticationToApprove"`
	WhenCommitAdded                            *string `yaml:"whenCommitAdded,omitempty" bson:"whenCommitAdded,omitempty" json:"whenCommitAdded,omitempty" mapstructure:"whenCommitAdded"`
}
