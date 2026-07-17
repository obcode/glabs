package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// Course files stopped going through viper's config-path loader, which expanded
// $HOME and ~ implicitly. findCourseFile has to do it explicitly, or a
// coursesfilepath like "$HOME/courses" is taken literally and the file is not
// found. This is a regression guard for exactly that.
func TestFindCourseFileExpandsHomeAndEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mpd.yaml"), []byte("mpd:\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", dir)
	t.Setenv("GLABS_TEST_DIR", dir)

	for _, coursesfilepath := range []string{dir, "$HOME", "~", "$GLABS_TEST_DIR"} {
		viper.Reset()
		viper.Set("coursesfilepath", coursesfilepath)

		got, err := findCourseFile("mpd")
		if err != nil {
			t.Errorf("coursesfilepath=%q: findCourseFile returned %v", coursesfilepath, err)
			continue
		}
		if want := filepath.Join(dir, "mpd.yaml"); got != want {
			t.Errorf("coursesfilepath=%q: findCourseFile = %q, want %q", coursesfilepath, got, want)
		}
	}
	viper.Reset()
}

func TestFindCourseFileMissing(t *testing.T) {
	viper.Reset()
	viper.Set("coursesfilepath", t.TempDir())
	t.Cleanup(viper.Reset)

	if _, err := findCourseFile("nosuchcourse"); err == nil {
		t.Fatal("findCourseFile(nosuchcourse) succeeded, want an error naming the file")
	}
}
