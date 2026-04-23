package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetReportGlobals(t *testing.T) {
	t.Helper()
	oldHTML := Html
	oldJSON := Json
	oldTemplate := Template
	oldExportTemplate := ExportTemplate
	oldOutput := OutPut
	t.Cleanup(func() {
		Html = oldHTML
		Json = oldJSON
		Template = oldTemplate
		ExportTemplate = oldExportTemplate
		OutPut = oldOutput
	})
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	if len(rootCmd.Commands()) == 0 {
		t.Fatal("expected root command to have subcommands")
	}
}

func TestReportCmd_ArgsRequireTwoArgs(t *testing.T) {
	resetReportGlobals(t)
	ExportTemplate = false

	err := reportCmd.Args(reportCmd, []string{"course-only"})
	if err == nil {
		t.Fatal("expected args validation error")
	}
}

func TestReportCmd_ArgsAllowNoArgsWhenExportTemplate(t *testing.T) {
	resetReportGlobals(t)
	ExportTemplate = true

	if err := reportCmd.Args(reportCmd, nil); err != nil {
		t.Fatalf("Args() unexpected error: %v", err)
	}
}

func TestReportCmd_RunPanicsForHtmlAndJSONTogether(t *testing.T) {
	resetReportGlobals(t)
	Html = true
	Json = true

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when --html and --json are both set")
		}
	}()

	reportCmd.Run(reportCmd, []string{"mpd", "blatt01"})
}

func TestReportCmd_ExportDefaultTemplateToFile(t *testing.T) {
	resetReportGlobals(t)
	ExportTemplate = true
	Html = true

	out := filepath.Join(t.TempDir(), "default-report-template.html")
	OutPut = out

	reportCmd.Run(reportCmd, nil)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading template output file failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected template output file to be non-empty")
	}
	if !strings.Contains(string(data), "<html") {
		t.Fatalf("expected HTML template content, got: %q", string(data))
	}
}
