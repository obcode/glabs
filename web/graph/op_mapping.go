package graph

import (
	"strconv"
	"strings"

	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// opToString maps the GraphQL Op enum (SETACCESS) onto the app's lowercase op name
// (setaccess).
func opToString(op model.Op) string {
	return strings.ToLower(string(op))
}

// paramsToMap flattens the op parameters into the string map the app uses. Only
// set fields are included, so unset ones fall back to the assignment config.
func paramsToMap(p *model.OpParams) map[string]string {
	if p == nil {
		return nil
	}
	m := map[string]string{}
	if p.AccessLevel != nil {
		m["accessLevel"] = *p.AccessLevel
	}
	if p.Branch != nil {
		m["branch"] = *p.Branch
	}
	if p.Unarchive != nil {
		m["unarchive"] = strconv.FormatBool(*p.Unarchive)
	}
	if p.SkipInvite != nil {
		m["skipInvite"] = strconv.FormatBool(*p.SkipInvite)
	}
	return m
}

// toGraphOpPlan projects an operation plan onto the GraphQL model.
func toGraphOpPlan(plan *app.OpPlan) *model.OpPlan {
	targets := make([]*model.PlannedTarget, 0, len(plan.Targets))
	for _, t := range plan.Targets {
		targets = append(targets, &model.PlannedTarget{For: t.For, Repo: t.Repo, URL: t.URL})
	}
	warnings := plan.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return &model.OpPlan{
		Op:            model.Op(strings.ToUpper(plan.Op)),
		Course:        plan.Course,
		Assignment:    plan.Assignment,
		Resolved:      plan.Resolved,
		Targets:       targets,
		Warnings:      warnings,
		Destructive:   plan.Destructive,
		ConfirmPhrase: emptyToNil(plan.ConfirmPhrase),
		Token:         plan.Token,
		ExpiresAt:     plan.ExpiresAt,
	}
}
