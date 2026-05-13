package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

func VersionCmd(ctx context.Context, appTitle string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Show the %s component versions", appTitle),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Only set KUBECONFIG if running as root and a cluster exists
			if isRoot() {
				rc := rcutil.InitBestRuntimeConfig(cmd.Context())
				_ = rc.SetEnv()
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			writer := table.NewWriter()
			writer.AppendHeader(table.Row{"component", "version"})

			channelRelease := release.GetChannelRelease()
			componentVersions, orderedKeys := collectBinaryVersions(channelRelease)

			for _, k := range orderedKeys {
				writer.AppendRow(table.Row{k, componentVersions[k]})
			}
			fmt.Printf("%s\n", writer.Render())
			return nil
		},
	}

	cmd.AddCommand(VersionMetadataCmd(ctx))
	cmd.AddCommand(VersionEmbeddedDataCmd(ctx))
	cmd.AddCommand(VersionListImagesCmd(ctx))

	return cmd
}

// collectBinaryVersions gathers all component versions from the binary.
// Returns a map of component name to version string, and an ordered slice of keys
// that matches the V2 display order (app, installer, kubernetes, then addons alphabetically).
func collectBinaryVersions(channelRelease *release.ChannelRelease) (map[string]string, []string) {
	componentVersions := make(map[string]string)
	orderedKeys := []string{}

	// Add app version from binary's channel release (first)
	if channelRelease != nil {
		appSlug := runtimeconfig.AppSlug()
		componentVersions[appSlug] = channelRelease.VersionLabel
		orderedKeys = append(orderedKeys, appSlug)
	}

	// Add Installer version (second)
	componentVersions["Installer"] = versions.Version
	orderedKeys = append(orderedKeys, "Installer")

	// Add Kubernetes version with (bundled) suffix (third)
	componentVersions["Kubernetes (bundled)"] = versions.K0sVersion
	orderedKeys = append(orderedKeys, "Kubernetes (bundled)")

	// Collect addon and extension versions
	addonKeys := []string{}
	collectAndNormalizeVersions(addons.Versions(), componentVersions, &addonKeys)
	collectAndNormalizeVersions(extensions.Versions(), componentVersions, &addonKeys)

	// Sort addon/extension keys alphabetically and append to ordered list
	sort.Strings(addonKeys)
	orderedKeys = append(orderedKeys, addonKeys...)

	return componentVersions, orderedKeys
}

// collectAndNormalizeVersions adds versions from source map to target map, normalizing version strings
// to include "v" prefix if missing, and appends keys to the provided slice.
func collectAndNormalizeVersions(source map[string]string, target map[string]string, keys *[]string) {
	for k, v := range source {
		if !strings.HasPrefix(v, "v") {
			v = fmt.Sprintf("v%s", v)
		}
		_, exists := target[k]
		target[k] = v
		if !exists {
			*keys = append(*keys, k)
		}
	}
}

// printVersionSection prints a version section with the given header and component versions.
// If orderedKeys is provided, components are printed in that order.
// If orderedKeys is nil, components are sorted alphabetically.
func printVersionSection(header string, componentVersions map[string]string, orderedKeys []string) {
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))

	// Use provided order or sort alphabetically
	var keys []string
	if orderedKeys != nil {
		keys = orderedKeys
	} else {
		keys = make([]string, 0, len(componentVersions))
		for k := range componentVersions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	// Find the longest component name for alignment
	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	maxLen += 1 // Add 1 for padding + 1 space in format = 2 total spaces

	// Print each component with proper indentation and alignment
	for _, k := range keys {
		fmt.Printf("  %-*s %s\n", maxLen, k, componentVersions[k])
	}
}

// isRoot checks if the current process is running with root privileges.
func isRoot() bool {
	return os.Geteuid() == 0
}
