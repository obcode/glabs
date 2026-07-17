package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/logrusorgru/aurora/v4"
	"github.com/obcode/glabs/v2/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configLintCmd, configFmtCmd, configMigrateCmd)
	configFmtCmd.Flags().BoolVarP(&configWrite, "write", "w", false, "write the result back to the file instead of printing it")
	configMigrateCmd.Flags().BoolVarP(&configWrite, "write", "w", false, "write the result back to the file instead of printing it")
}

var configWrite bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and maintain course configuration files",
	Long: `Inspect and maintain course configuration files.

These commands work on the course file as written, without resolving it: they
never talk to GitLab and never need a token.`,
}

var configLintCmd = &cobra.Command{
	Use:   "lint [course...]",
	Short: "Report configuration that does not do what it looks like it does",
	Long: `Report configuration that does not do what it looks like it does.

glabs ignores unknown keys silently, so a typo looks exactly like a setting that
works. lint names them, along with deprecated spellings and settings that are
overridden by a newer block.

With no arguments, every course listed in the main config is linted.

Exits non-zero if any problem (as opposed to a mere deprecation) was found, so
it can gate a commit hook or CI.`,
	Run: func(cmd *cobra.Command, args []string) {
		problems := 0
		for _, course := range coursesToProcess(args) {
			path := courseFilePath(course)
			source, decoded, err := decodeCourseFile(path)
			if err != nil {
				er(err)
			}

			findings := config.Lint(source, decoded)
			if len(findings) == 0 {
				fmt.Printf("%s %s\n", aurora.Green("ok"), path)
				continue
			}

			fmt.Printf("%s\n", aurora.Bold(path))
			for _, f := range findings {
				label := aurora.Yellow(string(f.Severity))
				if f.Severity == config.SeverityProblem {
					label = aurora.Red(string(f.Severity))
					problems++
				}
				fmt.Printf("  %s %s\n      %s\n", label, aurora.Cyan(f.Path), f.Message)
			}
		}

		if problems > 0 {
			fmt.Println()
			fmt.Println(aurora.Red(fmt.Sprintf("%d problem(s) found", problems)))
			os.Exit(1)
		}
	},
}

var configFmtCmd = &cobra.Command{
	Use:   "fmt course [course...]",
	Short: "Rewrite a course file in canonical form",
	Long: `Rewrite a course file in canonical form.

Note that comments and key order are not preserved: the file is rebuilt from the
parsed configuration, not edited in place. Review the diff before committing.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, course := range args {
			rewriteCourseFile(courseFilePath(course))
		}
	},
}

var configMigrateCmd = &cobra.Command{
	Use:   "migrate course [course...]",
	Short: "Rewrite a course file, upgrading deprecated spellings",
	Long: `Rewrite a course file, upgrading deprecated spellings.

Deprecated key spellings are folded into their canonical field on read, and
polymorphic blocks are written back in their modern shape, so migrating is the
same operation as formatting. It is a separate command only because the intent
differs: fmt is cosmetic, migrate changes what the file says.

What it does NOT do is resolve superseded settings — if both startercode.devBranch
and a branches: block are present, migrate keeps both and lint tells you which one
wins. Deciding that is a judgement call, not a rewrite.

Comments and key order are not preserved. Review the diff before committing.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for _, course := range args {
			rewriteCourseFile(courseFilePath(course))
		}
	},
}

func rewriteCourseFile(path string) {
	source, _, err := decodeCourseFile(path)
	if err != nil {
		er(err)
	}

	encoded, err := config.EncodeCourse(source)
	if err != nil {
		er(err)
	}

	if !configWrite {
		fmt.Print(string(encoded))
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		er(err)
	}
	if err := os.WriteFile(path, encoded, info.Mode().Perm()); err != nil {
		er(err)
	}
	fmt.Printf("%s %s\n", aurora.Green("wrote"), path)
}

func decodeCourseFile(path string) (*config.CourseSource, *config.DecodeResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read course file: %w", err)
	}
	source, decoded, err := config.DecodeCourse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return source, decoded, nil
}

// coursesToProcess returns the requested courses, or every configured course
// when none were named.
func coursesToProcess(args []string) []string {
	if len(args) > 0 {
		return args
	}
	courses := viper.GetStringSlice("courses")
	if len(courses) == 0 {
		er("no courses configured; add a `courses:` list to the main config or name one explicitly")
	}
	return courses
}

// courseFilePath locates a course file the same way initConfig does: by name,
// under coursesfilepath, with the extensions viper would have accepted.
func courseFilePath(course string) string {
	dir := viper.GetString("coursesfilepath")
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, course+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	er(fmt.Sprintf("cannot find a course file for %q: looked for %s.yaml and %s.yml in %q",
		course, course, course, dir))
	return ""
}
