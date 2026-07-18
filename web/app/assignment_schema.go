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
	Kind        FieldKind
	Required    bool
	Deprecated  bool
	Example     string
	Options     []FieldOption
}

// AssignmentSchema returns the metadata for the assignment editor's fields.
func AssignmentSchema() []FieldMeta {
	return assignmentFields
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
}
