package graph

import (
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// Mappers for the assignment-editor types. They live here, not in a
// *.resolvers.go file, so gqlgen (which owns those) leaves them alone.

// emptyToNil maps an empty string to nil, for nullable GraphQL String fields.
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// toGraphAssignmentSchema projects the server-authoritative field metadata onto
// the GraphQL model.
func toGraphAssignmentSchema(fields []app.FieldMeta) []*model.FieldMeta {
	out := make([]*model.FieldMeta, 0, len(fields))
	for _, f := range fields {
		opts := make([]*model.FieldOption, 0, len(f.Options))
		for _, o := range f.Options {
			opts = append(opts, &model.FieldOption{Value: o.Value, Label: o.Label, Description: o.Description})
		}
		out = append(out, &model.FieldMeta{
			Key:         f.Key,
			Label:       f.Label,
			Description: f.Description,
			Kind:        model.FieldKind(f.Kind),
			Required:    f.Required,
			Deprecated:  f.Deprecated,
			Example:     emptyToNil(f.Example),
			Options:     opts,
		})
	}
	return out
}

// draftToMap turns the GraphQL draft input into the key→value map the app uses.
func draftToMap(draft []*model.FieldValueInput) map[string]string {
	m := make(map[string]string, len(draft))
	for _, d := range draft {
		if d != nil {
			m[d.Key] = d.Value
		}
	}
	return m
}

// toGraphValidationResult projects a validation result onto the GraphQL model.
func toGraphValidationResult(vr *app.ValidationResult) *model.ValidationResult {
	errs := vr.Errors
	if errs == nil {
		errs = []string{}
	}
	return &model.ValidationResult{
		Ok:           vr.OK,
		Errors:       errs,
		Resolved:     emptyToNil(vr.Resolved),
		ResolveError: emptyToNil(vr.ResolveError),
	}
}

// toGraphAssignmentView projects an assignment view onto the GraphQL model.
func toGraphAssignmentView(view *app.AssignmentView) *model.AssignmentView {
	own := make([]*model.FieldValue, 0, len(view.Own))
	for _, v := range view.Own {
		own = append(own, &model.FieldValue{Key: v.Key, Value: v.Value})
	}
	return &model.AssignmentView{
		Course:       view.Course,
		Name:         view.Name,
		Extends:      emptyToNil(view.Extends),
		Abstract:     view.Abstract,
		Own:          own,
		Resolved:     view.Resolved,
		ResolveError: emptyToNil(view.ResolveError),
	}
}
