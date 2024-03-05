package metrics

// if metrics are disabled, we won't send any events
var metricsDisabled = false

func DisableMetrics() {
	metricsDisabled = true
}
