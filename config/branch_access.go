package config

import (
	"fmt"
	"strings"
)

// BranchAccess is who may push to, or merge into, a protected branch. The values
// mirror GitLab's choices for a protected branch ("No one", "Developers +
// Maintainers", "Maintainers"); developer includes maintainers, as in GitLab.
type BranchAccess string

const (
	BranchAccessNoOne      BranchAccess = "noOne"
	BranchAccessDeveloper  BranchAccess = "developer"
	BranchAccessMaintainer BranchAccess = "maintainer"
)

// parseBranchAccess accepts the spellings case-insensitively and returns the
// canonical one. Empty means "not set" and is not an error.
func parseBranchAccess(value string) (BranchAccess, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "noone":
		return BranchAccessNoOne, nil
	case "developer":
		return BranchAccessDeveloper, nil
	case "maintainer":
		return BranchAccessMaintainer, nil
	default:
		return "", fmt.Errorf("%q is not a branch access level (use noOne, developer or maintainer)", value)
	}
}

// Protected reports whether glabs protects the branch at all.
func (b BranchRule) Protected() bool {
	return b.Protect || b.MergeOnly || b.Push != "" || b.Merge != ""
}

// AccessLevels returns who may push and who may merge on a protected branch.
// `protect` is maintainer/maintainer and `mergeOnly` is noOne/developer; an
// explicit `push` or `merge` overrides the respective half of either shorthand,
// and a half nobody set stays at maintainer, GitLab's own default.
func (b BranchRule) AccessLevels() (push, merge BranchAccess) {
	push, merge = BranchAccessMaintainer, BranchAccessMaintainer
	if b.MergeOnly {
		push, merge = BranchAccessNoOne, BranchAccessDeveloper
	}
	if b.Push != "" {
		push = BranchAccess(b.Push)
	}
	if b.Merge != "" {
		merge = BranchAccess(b.Merge)
	}
	return push, merge
}
