// Package reporter abstracts the progress output of long-running operations, so
// the same gitlab and git code can drive a terminal spinner in the CLI and a
// streamed log in the web server.
//
// It lives in its own package because config, git and gitlab all report
// progress, and a shared type in any one of them would pull the others into an
// import cycle. reporter imports nothing from the project.
package reporter

// Reporter receives the progress of an operation. The CLI's ConsoleReporter
// renders it as spinners and colored lines; the web server will implement one
// that streams LogLines to the browser.
//
// Callers keep coloring their own text (aurora.Sprintf(...)) and pass the result
// through Printf/Println, so a console reporter reproduces today's output
// verbatim and a stream reporter can forward the ANSI codes for the browser to
// render.
type Reporter interface {
	// Printf writes a formatted line. The format may contain ANSI color codes.
	Printf(format string, a ...any)
	// Println writes its arguments as a line.
	Println(a ...any)
	// Task starts a unit of work shown with a spinner on the console. End it via
	// the returned Task's Done or Fail; exactly one of those must be called.
	Task(description string) Task
}

// Task is one unit of work in progress. It maps to a single spinner on the
// console: a running description that ends in success or failure.
type Task interface {
	// Update changes the running message while the task is in progress.
	Update(message string)
	// Done ends the task successfully. An empty message keeps the description.
	Done(message string)
	// Fail ends the task as failed, showing message.
	Fail(message string)
}
