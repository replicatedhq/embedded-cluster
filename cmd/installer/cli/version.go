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
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func VersionCmd(ctx context.Context, appTitle string) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Show the %s component versions", appTitle),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize runtime config and set environment variables
			// This sets up KUBECONFIG for accessing the cluster
			rc = rcutil.InitBestRuntimeConfig(cmd.Context())
			// Ignore SetEnv error - if it fails, cluster access will fail gracefully
			// and we'll only show client versions without server versions
			_ = rc.SetEnv()

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv("ENABLE_V3") == "1" {
				return runVersionV3(ctx)
			}

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

// runVersionV3 implements the version command behavior for v3 (when ENABLE_V3=1).
// A CLIENT (Binary) section is always displayed, and a SERVER (Deployed) section is conditionally displayed based on cluster accessibility.
func runVersionV3(ctx context.Context) error {
	channelRelease := release.GetChannelRelease()
	binaryVersions, binaryOrder := collectBinaryVersions(channelRelease)
	deployedVersions, hasCluster := collectDeployedVersions(ctx)

	printVersionSection("CLIENT (Binary)", binaryVersions, binaryOrder)
	if hasCluster {
		fmt.Println()
		printVersionSection("SERVER (Deployed)", deployedVersions, nil)
		fmt.Println()
	}

	return nil
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
		target[k] = v
		*keys = append(*keys, k)
	}
}

// collectDeployedVersions gathers component versions from the deployed cluster.
// Returns a map of component name to version string and a boolean indicating if cluster is accessible.
// Assumes KUBECONFIG has been set by RuntimeConfig.SetEnv() in PreRunE.
func collectDeployedVersions(ctx context.Context) (map[string]string, bool) {
	componentVersions := make(map[string]string)

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return componentVersions, false
	}

	// Get deployed app version from the config-values secret label
	appSlug := runtimeconfig.AppSlug()
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return componentVersions, false
	}

	secret := &corev1.Secret{}
	if err := kcli.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("%s-config-values", appSlug),
		Namespace: kotsadmNamespace,
	}, secret); err != nil {
		return componentVersions, false
	}

	if appVersion := secret.Labels["app.kubernetes.io/version"]; appVersion != "" {
		componentVersions[appSlug] = appVersion
	}

	return componentVersions, true
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
