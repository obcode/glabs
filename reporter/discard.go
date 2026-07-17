package reporter

// DiscardReporter drops everything. It backs the CLI's --suppress mode (where
// only machine-readable output is wanted) and keeps tests quiet.
type DiscardReporter struct{}

func NewDiscardReporter() *DiscardReporter { return &DiscardReporter{} }

func (r *DiscardReporter) Printf(string, ...any) {}
func (r *DiscardReporter) Println(...any)        {}
func (r *DiscardReporter) Task(string) Task      { return discardTask{} }

// NopTask is a task that does nothing. Operation helpers use it for the
// "already inside a parent task, don't start my own spinner" case, so their body
// can call Update/Done/Fail unconditionally instead of branching on a spin flag.
func NopTask() Task { return discardTask{} }

type discardTask struct{}

func (discardTask) Update(string) {}
func (discardTask) Done(string)   {}
func (discardTask) Fail(string)   {}
