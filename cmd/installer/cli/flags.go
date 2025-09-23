package cli

import (
	"os"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	flagAnnotationTarget                = "replicated.com/target"
	flagAnnotationTargetValueLinux      = "linux"
	flagAnnotationTargetValueKubernetes = "kubernetes"
)

// TODO: Remove this and use defaultUsageTemplateV3 when kubernetes support is added
const (
	upgradeUsageTemplateV3Linux = `Usage:{{if .Runnable}}
{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
{{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}{{if (usesTargetFlagMenu .LocalFlags)}}

Flags:
{{(commonFlags .LocalFlags).FlagUsages | trimTrailingWhitespaces}}{{else}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
)

const (
	defaultUsageTemplateV3 = `Usage:{{if .Runnable}}
{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
{{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}{{if (usesTargetFlagMenu .LocalFlags)}}

Common Flags:

{{(commonFlags .LocalFlags).FlagUsages | trimTrailingWhitespaces}}

Linux‑Specific Flags:
  (Valid only with --target=linux)

{{(linuxFlags .LocalFlags).FlagUsages | trimTrailingWhitespaces}}

Kubernetes‑Specific Flags:
  (Valid only with --target=kubernetes)

{{(kubernetesFlags .LocalFlags).FlagUsages | trimTrailingWhitespaces}}{{else}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
)

func init() {
	cobra.AddTemplateFuncs(template.FuncMap{
		// usesTargetFlagMenu returns true if the target flag is present and the ENABLE_V3 environment variable is set.
		"usesTargetFlagMenu": func(flagSet *pflag.FlagSet) bool {
			if isV3Enabled() {
				return flagSet.Lookup("target") != nil
			}
			return false
		},
		// commonFlags returns a flag set with all flags that do not have a target annotation.
		"commonFlags": func(flagSet *pflag.FlagSet) *pflag.FlagSet {
			return filterFlagSetNoTarget(flagSet)
		},
		// linuxFlags returns a flag set with all flags that have the target annotation set to linux.
		"linuxFlags": func(flagSet *pflag.FlagSet) *pflag.FlagSet {
			return filterFlagSetByTarget(flagSet, flagAnnotationTargetValueLinux)
		},
		// kubernetesFlags returns a flag set with all flags that have the target annotation set to kubernetes.
		"kubernetesFlags": func(flagSet *pflag.FlagSet) *pflag.FlagSet {
			return filterFlagSetByTarget(flagSet, flagAnnotationTargetValueKubernetes)
		},
	})
}

func isV3Enabled() bool {
	return os.Getenv("ENABLE_V3") == "1"
}

func mustSetFlagTargetLinux(flags *pflag.FlagSet, name string) {
	mustSetFlagTarget(flags, name, flagAnnotationTargetValueLinux)
}

func mustSetFlagTargetKubernetes(flags *pflag.FlagSet, name string) {
	mustSetFlagTarget(flags, name, flagAnnotationTargetValueKubernetes)
}

func mustSetFlagTarget(flags *pflag.FlagSet, name string, target string) {
	err := flags.SetAnnotation(name, flagAnnotationTarget, []string{target})
	if err != nil {
		panic(err)
	}
}

func mustMarkFlagRequired(flags *pflag.FlagSet, name string) {
	err := cobra.MarkFlagRequired(flags, name)
	if err != nil {
		panic(err)
	}
}

func mustMarkFlagHidden(flags *pflag.FlagSet, name string) {
	err := flags.MarkHidden(name)
	if err != nil {
		panic(err)
	}
}

func mustMarkFlagDeprecated(flags *pflag.FlagSet, name string, deprecationMessage string) {
	err := flags.MarkDeprecated(name, deprecationMessage)
	if err != nil {
		panic(err)
	}
}

func filterFlagSetByTarget(flags *pflag.FlagSet, target string) *pflag.FlagSet {
	if flags == nil {
		return nil
	}
	next := pflag.NewFlagSet(flags.Name(), pflag.ContinueOnError)
	flags.VisitAll(func(flag *pflag.Flag) {
		for _, t := range flag.Annotations[flagAnnotationTarget] {
			if t == target {
				next.AddFlag(flag)
				break
			}
		}
	})
	return next
}

func filterFlagSetNoTarget(flags *pflag.FlagSet) *pflag.FlagSet {
	if flags == nil {
		return nil
	}
	next := pflag.NewFlagSet(flags.Name(), pflag.ContinueOnError)
	flags.VisitAll(func(flag *pflag.Flag) {
		if len(flag.Annotations[flagAnnotationTarget]) == 0 {
			next.AddFlag(flag)
		}
	})
	return next
}
