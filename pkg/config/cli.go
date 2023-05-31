package config

import (
	"fmt"
	"path/filepath"

	k0sconfig "github.com/k0sproject/k0s/pkg/config"
	k0sconstant "github.com/k0sproject/k0s/pkg/constant"
	"github.com/spf13/pflag"
)

// CLIOptions are common options for the CLI
type CLIOptions struct {
	DataDir string
}

// K0sOptions are common options for the controller and worker
type K0sOptions struct {
	Debug        bool
	CfgFile      string
	TokenFile    string
	CmdLogLevels map[string]string
}

// BinDir returns the path to the bin directory
func (o *CLIOptions) BinDir() string {
	return filepath.Join(o.DataDir, "bin")
}

// RunDir returns the path to the run directory
func (o *CLIOptions) RunDir() string {
	return "/run/replicated"
}

// ConfigFile returns the path to the rendered k0s config file
func (o *CLIOptions) ConfigFile() string {
	return filepath.Join(o.DataDir, "etc/k0s/config.yaml")
}

// K0sControllerOptions are the options for the controller
type K0sControllerOptions struct {
	CLIOptions
	K0sOptions
	EnableWorker bool
	NoTaints     bool
}

// K0sWorkerOptions are the options for the worker
type K0sWorkerOptions struct {
	CLIOptions
	K0sOptions
}

// GetCLIFlags returns the common flags for the CLI
func GetCLIFlags(opts *CLIOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	// TODO cobra auto completes files
	flagset.StringVar(
		&opts.DataDir, "data-dir", DataDirDefault,
		"Data Directory. DO NOT CHANGE for an existing setup, things will break!")
	return flagset
}

// GetK0sFlags returns the common flags for the controller and worker
func GetK0sFlags(opts *K0sOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.BoolVarP(&opts.Debug, "debug", "d", false, "Debug logging (default: false)")
	// TODO: read the config from stdin
	descString := fmt.Sprintf(
		"k0s config file, use '-' to read the config from stdin (default \"%s\")", k0sconstant.K0sConfigPathDefault)
	// TODO cobra auto completes files
	flagset.StringVarP(&opts.CfgFile, "config", "c", "", descString)
	// TODO cobra auto completes files
	flagset.StringVar(&opts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	flagset.StringToStringVarP(
		&opts.CmdLogLevels, "logging", "l", k0sconfig.DefaultLogLevels(), "Logging Levels for the different components")
	return flagset
}

// GetK0sControllerFlags returns the flags for the controller
func GetK0sControllerFlags(opts *K0sControllerOptions, singleNode bool) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.BoolVar(&opts.EnableWorker, "enable-worker", singleNode, "enable worker")
	flagset.BoolVar(&opts.NoTaints, "no-taints", singleNode, "disable default taints for controller node")
	flagset.AddFlagSet(GetCLIFlags(&opts.CLIOptions))
	flagset.AddFlagSet(GetK0sFlags(&opts.K0sOptions))
	return flagset
}

// GetK0sWorkerFlags returns the flags for the worker
func GetK0sWorkerFlags(opts *K0sWorkerOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.AddFlagSet(GetCLIFlags(&opts.CLIOptions))
	flagset.AddFlagSet(GetK0sFlags(&opts.K0sOptions))
	return flagset
}
