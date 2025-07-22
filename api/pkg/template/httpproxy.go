package template

func (e *Engine) httpProxy() string {
	spec := e.installationConfig.ProxySpec()
	if spec == nil {
		return ""
	}
	return spec.HTTPProxy
}

func (e *Engine) httpsProxy() string {
	spec := e.installationConfig.ProxySpec()
	if spec == nil {
		return ""
	}
	return spec.HTTPSProxy
}

func (e *Engine) noProxy() string {
	spec := e.installationConfig.ProxySpec()
	if spec == nil {
		return ""
	}
	return spec.NoProxy
}
