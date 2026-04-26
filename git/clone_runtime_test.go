package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/obcode/glabs/v2/config"
)

func TestClone_NoSpinner_CloneErrorNoPanic(t *testing.T) {
	resetViper(t)

	local := filepath.Join(t.TempDir(), "repo")
	clone(local, "main", "http://127.0.0.1:1/does-not-exist.git", nil, false, true)
}

func TestClone_Force_RemovesExistingPath(t *testing.T) {
	resetViper(t)

	local := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(local, 0755); err != nil {
		t.Fatalf("creating local dir failed: %v", err)
	}
	marker := filepath.Join(local, "marker.txt")
	if err := os.WriteFile(marker, []byte("x"), 0644); err != nil {
		t.Fatalf("writing marker file failed: %v", err)
	}

	clone(local, "main", "http://127.0.0.1:1/does-not-exist.git", nil, true, true)

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("expected marker file to be removed by force clone")
	}
}

func TestClone_PerStudent_PathAndDispatch(t *testing.T) {
	resetViper(t)

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		URL:    "https://127.0.0.1/group/path",
		Per:    config.PerStudent,
		Students: []*config.Student{
			{Username: &username, Raw: username},
		},
		Clone: &config.Clone{LocalPath: t.TempDir(), Branch: "main", Force: false},
	}

	Clone(cfg, true)
}

func TestClone_PerGroup_PathAndDispatch(t *testing.T) {
	resetViper(t)

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		URL:    "https://127.0.0.1/group/path",
		Per:    config.PerGroup,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{}},
		},
		Clone: &config.Clone{LocalPath: t.TempDir(), Branch: "main", Force: false},
	}

	Clone(cfg, true)
}
