package reporter

import "testing"

// The discard reporter must absorb everything without panicking, including a
// full task lifecycle — it backs --suppress and tests, where any output or a nil
// task would be a bug.
func TestDiscardReporter(t *testing.T) {
	r := NewDiscardReporter()
	r.Printf("%s", "ignored")
	r.Println("ignored")

	task := r.Task("work")
	if task == nil {
		t.Fatal("Task returned nil")
	}
	task.Update("still working")
	task.Done("done")
	task.Fail("also fine after done")
}

// The console reporter must tolerate a headless environment: creating a spinner
// can fail with no TTY, and the task must degrade to no-ops rather than panic on
// a nil spinner.
func TestConsoleReporterTaskDoesNotPanicHeadless(t *testing.T) {
	r := NewConsoleReporter()

	task := r.Task("work")
	if task == nil {
		t.Fatal("Task returned nil")
	}
	task.Update("progress")
	task.Done("")

	// A fresh task ended via Fail must be fine too.
	r.Task("other").Fail("nope")
}
