package migrate

import (
	"fmt"
	"io"
)

// printf and println write progress to the operator's terminal.
//
// The write error is dropped on purpose, and in one place rather than at six
// call sites: if stdout has gone away there is nothing useful left to do about
// it, and threading an error through every progress line would bury the actual
// work in plumbing. What must not be dropped is an error from the migration
// itself, and those are returned.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func println(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
