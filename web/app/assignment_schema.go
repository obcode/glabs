package app

// The assignment editor is schema-driven: the GUI renders its form from this
// server-authoritative metadata, so labels, help text and dropdown options live
// in exactly one place (here) and never drift between backend and frontend.
//
// This is deliberately a curated, hand-written registry rather than reflection
// over the config structs: the value is the short, human descriptions and the
// per-option help, which reflection cannot produce. It starts with the core
// top-level fields and grows block by block (mergeRequest, branches, …).

// FieldKind is the input shape the GUI should render for a field.
type FieldKind string

const (
	KindString     FieldKind = "STRING"
	KindBool       FieldKind = "BOOL"
	KindEnum       FieldKind = "ENUM"
	KindInt        FieldKind = "INT"
	KindStringList FieldKind = "STRINGLIST"
)

// FieldOption is one choice of an ENUM field — a dropdown entry with its own
// short description.
type FieldOption struct {
	Value       string
	Label       string
	Description string
}

// FieldMeta describes one editable assignment field.
type FieldMeta struct {
	Key         string
	Label       string
	Description string
	// Group is the section the field belongs to ("" for the top-level group,
	// e.g. "Startercode" for the startercode block), so the GUI can render
	// grouped sections. Nested keys are dotted, e.g. "startercode.url".
	Group      string
	Kind       FieldKind
	Required   bool
	Deprecated bool
	Example    string
	Options    []FieldOption
}

// AssignmentSchema returns the metadata for the assignment editor's fields.
func AssignmentSchema() []FieldMeta {
	return assignmentFields
}

// AssignmentBranchSchema returns the metadata for one branch rule — a row of the
// repeat-group `branches` list. The GUI renders one such control set per row.
func AssignmentBranchSchema() []FieldMeta {
	return branchFields
}

// AssignmentApprovalSettingsSchema returns the metadata for the mergeRequest
// approval settings. These are tri-state: an ENUM whose empty option means
// "not set" (inherit), so `an`/`aus` map to *bool true/false and "" to nil.
func AssignmentApprovalSettingsSchema() []FieldMeta {
	return approvalSettingsFields
}

// AssignmentApprovalRuleSchema returns the metadata for one approval rule — a row
// of the repeat-group `approvals.rules` list.
func AssignmentApprovalRuleSchema() []FieldMeta {
	return approvalRuleFields
}

var triStateOptions = []FieldOption{
	{Value: "true", Label: "an", Description: "aktiviert"},
	{Value: "false", Label: "aus", Description: "deaktiviert"},
}

var approvalSettingsFields = []FieldMeta{
	{
		Key:         "preventApprovalByMergeRequestCreator",
		Label:       "Ersteller darf nicht selbst freigeben",
		Description: "Die MR-Erstellerin/der Ersteller kann den eigenen MR nicht approven.",
		Kind:        KindEnum,
		Options:     triStateOptions,
	},
	{
		Key:         "preventApprovalsByUsersWhoAddCommits",
		Label:       "Committer dürfen nicht freigeben",
		Description: "Wer Commits hinzufügt, kann den MR nicht approven.",
		Kind:        KindEnum,
		Options:     triStateOptions,
	},
	{
		Key:         "preventEditingApprovalRulesInMergeRequests",
		Label:       "Approval-Regeln im MR sperren",
		Description: "Approval-Regeln können im einzelnen MR nicht geändert werden.",
		Kind:        KindEnum,
		Options:     triStateOptions,
	},
	{
		Key:         "requireUserReauthenticationToApprove",
		Label:       "Re-Authentifizierung zum Freigeben",
		Description: "Approven erfordert erneute Authentifizierung.",
		Kind:        KindEnum,
		Options:     triStateOptions,
	},
	{
		Key:         "whenCommitAdded",
		Label:       "Bei neuem Commit",
		Description: "Was mit bestehenden Approvals passiert, wenn ein Commit hinzukommt (leer = GitLab-Default).",
		Kind:        KindEnum,
		Options: []FieldOption{
			{Value: "keepApprovals", Label: "Behalten", Description: "Approvals bleiben bestehen."},
			{Value: "removeAllApprovals", Label: "Alle entfernen", Description: "Alle Approvals werden zurückgesetzt."},
			{Value: "removeCodeOwnerApprovalsIfTheirFilesChanged", Label: "Code-Owner (geänderte Dateien)", Description: "Nur Code-Owner-Approvals entfernen, wenn deren Dateien geändert wurden."},
		},
	},
}

var approvalRuleFields = []FieldMeta{
	{
		Key:         "name",
		Label:       "Regelname",
		Description: "Name der Approval-Regel.",
		Kind:        KindString,
		Required:    true,
		Example:     "Tutoren",
	},
	{
		Key:         "requiredApprovals",
		Label:       "Erforderliche Freigaben",
		Description: "Anzahl nötiger Approvals für diese Regel.",
		Kind:        KindInt,
		Example:     "1",
	},
	{
		Key:         "usernames",
		Label:       "Usernames",
		Description: "GitLab-Usernames, die freigeben dürfen (kommagetrennt).",
		Kind:        KindStringList,
	},
	{
		Key:         "groups",
		Label:       "Gruppen",
		Description: "GitLab-Gruppen(pfade), die freigeben dürfen (kommagetrennt).",
		Kind:        KindStringList,
	},
	{
		Key:         "branches",
		Label:       "Branches",
		Description: "Branches, für die die Regel gilt (kommagetrennt; leer = alle).",
		Kind:        KindStringList,
	},
	{
		Key:         "multiMemberGroupsOnly",
		Label:       "Nur mehrköpfige Gruppen",
		Description: "Nur Gruppen mit mehr als einem Mitglied zählen.",
		Kind:        KindBool,
	},
}

var branchFields = []FieldMeta{
	{
		Key:         "name",
		Label:       "Branch",
		Description: "Name des Branches, z. B. main.",
		Kind:        KindString,
		Required:    true,
		Example:     "main",
	},
	{
		Key:         "protect",
		Label:       "Geschützt",
		Description: "Branch schützen (Push-Regeln, kein versehentliches Löschen).",
		Kind:        KindBool,
	},
	{
		Key:         "mergeOnly",
		Label:       "Nur via Merge",
		Description: "Änderungen nur über Merge-Requests, kein direkter Push.",
		Kind:        KindBool,
	},
	{
		Key:         "default",
		Label:       "Default-Branch",
		Description: "Diesen Branch als Standard-Branch des Repos setzen.",
		Kind:        KindBool,
	},
	{
		Key:         "allowForcePush",
		Label:       "Force-Push erlauben",
		Description: "Force-Push auf den geschützten Branch zulassen.",
		Kind:        KindBool,
	},
	{
		Key:         "codeOwnerApprovalRequired",
		Label:       "Code-Owner-Approval",
		Description: "Freigabe durch Code-Owner erforderlich.",
		Kind:        KindBool,
	},
}

var assignmentFields = []FieldMeta{
	{
		Key:         "extends",
		Label:       "Erbt von",
		Description: "Name eines anderen Assignments, dessen Einstellungen als Basis übernommen werden. Eigene Angaben überschreiben die geerbten.",
		Kind:        KindString,
		Example:     "defaults",
	},
	{
		Key:         "abstract",
		Label:       "Abstrakt (nur Basis)",
		Description: "Reine Vorlage zum Erben. Ein abstraktes Assignment lässt sich nicht direkt generieren.",
		Kind:        KindBool,
	},
	{
		Key:         "per",
		Label:       "Pro",
		Description: "Ob je Studierender:m oder je Gruppe ein Repository erzeugt wird.",
		Kind:        KindEnum,
		Required:    true,
		Options: []FieldOption{
			{Value: "student", Label: "Studierende:r", Description: "Ein Repository pro Person."},
			{Value: "group", Label: "Gruppe", Description: "Ein Repository pro Gruppe."},
		},
	},
	{
		Key:         "accesslevel",
		Label:       "Zugriffsrecht",
		Description: "Recht, das die Studierenden auf ihrem eigenen Repository erhalten.",
		Kind:        KindEnum,
		Options: []FieldOption{
			{Value: "guest", Label: "Guest", Description: "Nur Issues anlegen/sehen, kein Code-Zugriff."},
			{Value: "reporter", Label: "Reporter", Description: "Code lesen, aber nicht pushen."},
			{Value: "developer", Label: "Developer", Description: "Pushen und Merge-Requests — der Normalfall."},
			{Value: "maintainer", Label: "Maintainer", Description: "Volle Verwaltung inkl. Einstellungen."},
		},
	},
	{
		Key:         "description",
		Label:       "Beschreibung",
		Description: "Kurzbeschreibung des Assignments; landet in der Projektbeschreibung der erzeugten Repositories.",
		Kind:        KindString,
	},
	{
		Key:         "assignmentpath",
		Label:       "Assignment-Pfad",
		Description: "Pfadsegment unter dem Kurs-/Semesterpfad. Leer lassen = der Assignment-Name wird verwendet.",
		Kind:        KindString,
		Example:     "blatt01",
	},
	{
		Key:         "containerRegistry",
		Label:       "Container-Registry",
		Description: "Container-Registry für die erzeugten Projekte aktivieren.",
		Kind:        KindBool,
	},

	// --- Startercode: aus welchem Repo/Branch die Studi-Repos befüllt werden ---
	{
		Key:         "startercode.url",
		Label:       "Startercode-URL",
		Description: "Git-URL des Startercode-Repos. SSH-Notation (git@host:pfad.git) ist erlaubt und wird zu HTTPS aufgelöst.",
		Group:       "Startercode",
		Kind:        KindString,
		Example:     "git@gitlab.lrz.de:kurs/startercode/blatt-01.git",
	},
	{
		Key:         "startercode.fromBranch",
		Label:       "Von Branch",
		Description: "Branch im Startercode, aus dem befüllt wird (Standard: der Default-Branch).",
		Group:       "Startercode",
		Kind:        KindString,
		Example:     "main",
	},
	{
		Key:         "startercode.tag",
		Label:       "Tag",
		Description: "Statt eines Branches einen bestimmten Tag verwenden.",
		Group:       "Startercode",
		Kind:        KindString,
	},
	{
		Key:         "startercode.toBranch",
		Label:       "Nach Branch",
		Description: "Zielbranch im Studi-Repo, in den der Startercode gelegt wird (Standard: main).",
		Group:       "Startercode",
		Kind:        KindString,
		Example:     "main",
	},
	{
		Key:         "startercode.template",
		Label:       "Als Template",
		Description: "Den ersten Commit als Vorlage-Commit markieren.",
		Group:       "Startercode",
		Kind:        KindBool,
	},
	{
		Key:         "startercode.templateMessage",
		Label:       "Template-Commit-Nachricht",
		Description: "Commit-Nachricht für den Vorlage-Commit.",
		Group:       "Startercode",
		Kind:        KindString,
	},
	{
		Key:         "startercode.additionalBranches",
		Label:       "Zusätzliche Branches",
		Description: "Weitere Branches, die zusätzlich angelegt werden. Kommagetrennt.",
		Group:       "Startercode",
		Kind:        KindStringList,
		Example:     "dev, test",
	},

	// --- Merge-Request: Merge-Strategie und -Bedingungen der Studi-Repos ---
	{
		Key:         "mergeRequest.mergeMethod",
		Label:       "Merge-Methode",
		Description: "Wie Merge-Requests zusammengeführt werden.",
		Group:       "Merge-Request",
		Kind:        KindEnum,
		Options: []FieldOption{
			{Value: "merge", Label: "Merge-Commit", Description: "Klassischer Merge-Commit — erhält die Historie."},
			{Value: "semi_linear", Label: "Semi-linear", Description: "Merge-Commit, aber nur bei aktuellem Zielbranch (Rebase nötig)."},
			{Value: "ff", Label: "Fast-Forward", Description: "Keine Merge-Commits, streng lineare Historie."},
		},
	},
	{
		Key:         "mergeRequest.squashOption",
		Label:       "Squash-Option",
		Description: "Ob Commits beim Merge zu einem zusammengefasst werden.",
		Group:       "Merge-Request",
		Kind:        KindEnum,
		Options: []FieldOption{
			{Value: "never", Label: "Nie", Description: "Nie squashen."},
			{Value: "always", Label: "Immer", Description: "Immer squashen."},
			{Value: "default_off", Label: "Standard aus", Description: "Pro MR wählbar, vorausgewählt: aus."},
			{Value: "default_on", Label: "Standard an", Description: "Pro MR wählbar, vorausgewählt: an."},
		},
	},
	{
		Key:         "mergeRequest.pipeline",
		Label:       "Pipeline muss erfolgreich sein",
		Description: "Merge nur erlauben, wenn die Pipeline durchläuft.",
		Group:       "Merge-Request",
		Kind:        KindBool,
	},
	{
		Key:         "mergeRequest.skippedPipelinesAreSuccessful",
		Label:       "Übersprungene Pipelines gelten als erfolgreich",
		Description: "Eine übersprungene Pipeline blockiert den Merge nicht.",
		Group:       "Merge-Request",
		Kind:        KindBool,
	},
	{
		Key:         "mergeRequest.allThreadsMustBeResolved",
		Label:       "Alle Threads müssen aufgelöst sein",
		Description: "Merge nur erlauben, wenn alle Diskussionen aufgelöst sind.",
		Group:       "Merge-Request",
		Kind:        KindBool,
	},
	{
		Key:         "mergeRequest.statusChecksMustSucceed",
		Label:       "Status-Checks müssen erfolgreich sein",
		Description: "Merge nur erlauben, wenn alle externen Status-Checks grün sind.",
		Group:       "Merge-Request",
		Kind:        KindBool,
	},

	// --- Issues: welche Issues aus dem Startercode in die Studi-Repos kommen ---
	{
		Key:         "issues.replicateFromStartercode",
		Label:       "Issues aus Startercode übernehmen",
		Description: "Issues des Startercode-Repos in die erzeugten Repos replizieren.",
		Group:       "Issues",
		Kind:        KindBool,
	},
	{
		Key:         "issues.issueNumbers",
		Label:       "Issue-Nummern",
		Description: "Nur diese Issue-Nummern übernehmen (kommagetrennt). Leer = alle.",
		Group:       "Issues",
		Kind:        KindStringList,
		Example:     "1, 2, 5",
	},
	{
		Key:         "issues.includeChildTasks",
		Label:       "Unteraufgaben einschließen",
		Description: "Auch die Child-Tasks der übernommenen Issues replizieren.",
		Group:       "Issues",
		Kind:        KindBool,
	},
}
