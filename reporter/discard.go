package reporter

// DiscardReporter drops everything. It backs the CLI's --suppress mode (where
// only machine-readable output is wanted) and keeps tests quiet.
type DiscardReporter struct{}

func NewDiscardReporter() *DiscardReporter { return &DiscardReporter{} }

func (r *DiscardReporter) Printf(string, ...any) {}
func (r *DiscardReporter) Println(...any)        {}
func (r *DiscardReporter) Task(string) Task      { return discardTask{} }

type discardTask struct{}

func (discardTask) Update(string) {}
func (discardTask) Done(string)   {}
func (discardTask) Fail(string)   {}
