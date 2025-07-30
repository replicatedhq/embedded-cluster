package template

func (e *Engine) httpProxy() string {
	return e.execOptions.httpProxy
}

func (e *Engine) httpsProxy() string {
	return e.execOptions.httpsProxy
}

func (e *Engine) noProxy() string {
	return e.execOptions.noProxy
}
