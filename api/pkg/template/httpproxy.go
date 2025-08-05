package template

// These methods require a valid proxy spec to be set in the engine and that will only happen in generic mode. We add guardrails to prevent misuse in the `Execute` method.

func (e *Engine) httpProxy() string {
	if e.proxySpec == nil {
		return ""
	}
	return e.proxySpec.HTTPProxy
}

func (e *Engine) httpsProxy() string {
	if e.proxySpec == nil {
		return ""
	}
	return e.proxySpec.HTTPSProxy
}

func (e *Engine) noProxy() string {
	if e.proxySpec == nil {
		return ""
	}
	return e.proxySpec.NoProxy
}
