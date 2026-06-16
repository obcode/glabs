package config

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// When a project is created without an explicit path, GitLab derives the path
// from the project name (see Projects::CreateService#set_project_name_from_path):
//
//	@project.path = @project.name.dup
//	unless @project.path.match?(Gitlab::PathRegex.project_path_format_regex)
//	  @project.path = @project.path.parameterize
//	end
//
// So the name is kept verbatim as the path *if* it already is a valid project
// path; only otherwise is it run through Rails' String#parameterize. This is
// why a "." survives in "alice_at_hm.edu" but a name containing a "+" (which is
// invalid in a path) is slugified as a whole — turning every "." into "-" and
// lowercasing the result too. We mirror that exact logic in gitlabProjectPath so
// the URLs we print and the paths we search for match the repositories GitLab
// actually creates.
var (
	nonSlugChars  = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	duplicateDash = regexp.MustCompile(`-{2,}`)
	// stripDiacritics decomposes runes (NFD) and removes the resulting
	// combining marks, turning e.g. "ä" into "a", matching Rails' default
	// transliteration.
	stripDiacritics = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	// projectPathFormatRegex mirrors Gitlab::PathRegex.project_path_format_regex:
	// first char [a-zA-Z0-9_.], remaining chars [a-zA-Z0-9_-.]. The Ruby regex
	// additionally forbids a ".git"/".atom" suffix via a negative lookbehind,
	// which RE2 cannot express, so that part is checked separately.
	projectPathFormatRegex = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9_.-]*$`)
)

// gitlabProjectPath returns the path GitLab assigns to a project created with
// the given name and no explicit path.
func gitlabProjectPath(name string) string {
	if projectPathFormatRegex.MatchString(name) &&
		!strings.HasSuffix(name, ".git") &&
		!strings.HasSuffix(name, ".atom") {
		return name
	}
	return parameterize(name)
}

// asciiLigatures handles characters that have no NFD decomposition but that
// Rails' default transliteration still folds to ASCII (e.g. "ß" -> "ss").
var asciiLigatures = strings.NewReplacer(
	"ß", "ss",
	"ẞ", "SS",
)

// parameterize mirrors GitLab's project name-to-path conversion so that the
// path computed by glabs matches the path GitLab assigns to the project.
func parameterize(s string) string {
	s = asciiLigatures.Replace(s)
	if transliterated, _, err := transform.String(stripDiacritics, s); err == nil {
		s = transliterated
	}
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = duplicateDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return strings.ToLower(s)
}
