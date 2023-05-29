package config

import "github.com/spf13/pflag"

// ControllerOptions are the options for the controller
type ControllerOptions struct {
	EnableWorker bool
	NoTaints     bool
}

// WorkerOptions are the options for the worker
type WorkerOptions struct {
	TokenFile string
}

// GetControllerFlags returns the flags for the controller
func GetControllerFlags(controllerOpts *ControllerOptions, workerOpts *WorkerOptions, singleNode bool) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.BoolVar(&controllerOpts.EnableWorker, "enable-worker", singleNode, "enable worker")
	flagset.BoolVar(&controllerOpts.NoTaints, "no-taints", singleNode, "disable default taints for controller node")
	// TODO cobra auto completes files
	flagset.StringVar(&workerOpts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	return flagset
}

// GetWorkerFlags returns the flags for the worker
func GetWorkerFlags(workerOpts *WorkerOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	// TODO cobra auto completes files
	flagset.StringVar(&workerOpts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	return flagset
}
