package preflights

var _ PreflightRunnerInterface = (*PreflightRunner)(nil)

type PreflightRunner struct {
}

type PreflightsOption func(*PreflightRunner)

func New(opts ...PreflightsOption) *PreflightRunner {
	h := &PreflightRunner{}

	for _, opt := range opts {
		opt(h)
	}

	return h
}
