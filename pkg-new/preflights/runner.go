package preflights

var _ PreflightsRunnerInterface = (*PreflightsRunner)(nil)

type PreflightsRunner struct {
}

type PreflightsOption func(*PreflightsRunner)

func New(opts ...PreflightsOption) *PreflightsRunner {
	h := &PreflightsRunner{}

	for _, opt := range opts {
		opt(h)
	}

	return h
}
