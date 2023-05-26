package config

import "github.com/spf13/pflag"

// ControllerOptions are the options for the controller
type ControllerOptions struct {
	EnableWorker bool
	NoTaints     bool
	TokenFile    string
}

// WorkerOptions are the options for the worker
type WorkerOptions struct {
	TokenFile string
}

// GetControllerFlags returns the flags for the controller
func GetControllerFlags(opts *ControllerOptions, singleNode bool) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.BoolVar(&opts.EnableWorker, "enable-worker", singleNode, "enable worker")
	flagset.BoolVar(&opts.NoTaints, "no-taints", singleNode, "disable default taints for controller node")
	// TODO cobra auto completes files
	flagset.StringVar(&opts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	return flagset
}

// GetWorkerFlags returns the flags for the worker
func GetWorkerFlags(opts *WorkerOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	// TODO cobra auto completes files
	flagset.StringVar(&opts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	return flagset
}
