package reporter

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/theckman/yacspin"
)

// ConsoleReporter is the CLI reporter: spinners for tasks, stdout for lines. It
// reproduces the terminal output glabs had before the reporter existed, so the
// yacspin configuration here matches what the operation code used inline.
type ConsoleReporter struct{}

func NewConsoleReporter() *ConsoleReporter { return &ConsoleReporter{} }

func (r *ConsoleReporter) Printf(format string, a ...any) { fmt.Printf(format, a...) }
func (r *ConsoleReporter) Println(a ...any)               { fmt.Println(a...) }

func (r *ConsoleReporter) Task(description string) Task {
	cfg := yacspin.Config{
		Frequency:         100 * time.Millisecond,
		CharSet:           yacspin.CharSets[69],
		Suffix:            description,
		SuffixAutoColon:   true,
		StopCharacter:     "✓",
		StopColors:        []string{"fgGreen"},
		StopFailMessage:   "error",
		StopFailCharacter: "✗",
		StopFailColors:    []string{"fgRed"},
	}

	spinner, err := yacspin.New(cfg)
	if err != nil {
		log.Debug().Err(err).Msg("cannot create spinner")
		return &consoleTask{}
	}
	if err := spinner.Start(); err != nil {
		log.Debug().Err(err).Msg("cannot start spinner")
	}
	return &consoleTask{spinner: spinner}
}

// consoleTask wraps a yacspin spinner. spinner may be nil if creation failed, in
// which case the task degrades to no-ops — the same tolerance the inline spinner
// code had.
type consoleTask struct {
	spinner *yacspin.Spinner
}

func (t *consoleTask) Update(message string) {
	if t.spinner == nil {
		return
	}
	t.spinner.Message(message)
}

func (t *consoleTask) Done(message string) {
	if t.spinner == nil {
		return
	}
	if message != "" {
		t.spinner.StopMessage(message)
	}
	if err := t.spinner.Stop(); err != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}
}

func (t *consoleTask) Fail(message string) {
	if t.spinner == nil {
		return
	}
	if message != "" {
		t.spinner.StopFailMessage(message)
	}
	if err := t.spinner.StopFail(); err != nil {
		log.Debug().Err(err).Msg("cannot stop spinner")
	}
}
