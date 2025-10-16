package template

// isAirgap returns true if this is an airgap installation
func (e *Engine) isAirgap() bool {
	return e.isAirgapInstallation
}
