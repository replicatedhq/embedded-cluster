package cli

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/k0sproject/dig"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func JoinRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle string
		license      string
		noPrompt     bool
	)
	cmd := &cobra.Command{
		Use:   "run-preflights",
		Short: fmt.Sprintf("Run join host preflights for %s", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("run-preflights command must be run as root")
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("usage: %s join preflights <url> <token>", name)
			}

			logrus.Debugf("fetching join token remotely")
			jcmd, err := getJoinToken(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("unable to get join token: %w", err)
			}

			runtimeconfig.Set(jcmd.InstallationSpec.RuntimeConfig)
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			// check to make sure the version returned by the join token is the same as the one we are running
			if strings.TrimPrefix(jcmd.EmbeddedClusterVersion, "v") != strings.TrimPrefix(versions.Version, "v") {
				return fmt.Errorf("embedded cluster version mismatch - this binary is version %q, but the cluster is running version %q", versions.Version, jcmd.EmbeddedClusterVersion)
			}

			setProxyEnv(jcmd.InstallationSpec.Proxy)

			networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
			if err != nil {
				return fmt.Errorf("unable to get network-interface flag: %w", err)
			}
			proxyOK, localIP, err := checkProxyConfigForLocalIP(jcmd.InstallationSpec.Proxy, networkInterfaceFlag)
			if err != nil {
				return fmt.Errorf("failed to check proxy config for local IP: %w", err)
			}
			if !proxyOK {
				return fmt.Errorf("no-proxy config %q does not allow access to local IP %q", jcmd.InstallationSpec.Proxy.NoProxy, localIP)
			}

			isAirgap := false
			if airgapBundle != "" {
				isAirgap = true
			}
			logrus.Debugf("materializing binaries")
			if err := materializeFiles(airgapBundle); err != nil {
				return err
			}

			if err := configutils.ConfigureSysctl(); err != nil {
				return err
			}

			opts := addonsApplierOpts{
				noPrompt:     noPrompt,
				license:      "",
				airgapBundle: airgapBundle,
				overrides:    "",
				privateCAs:   nil,
				configValues: "",
			}
			applier, err := getAddonsApplier(cmd, opts, "", jcmd.InstallationSpec.Proxy)
			if err != nil {
				return err
			}

			fromCIDR, toCIDR, err := DeterminePodAndServiceCIDRs(cmd)
			if err != nil {
				return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
			}

			logrus.Debugf("running host preflights")
			replicatedAPIURL := jcmd.InstallationSpec.MetricsBaseURL
			proxyRegistryURL := fmt.Sprintf("https://%s", runtimeconfig.ProxyRegistryAddress)
			if err := RunHostPreflights(cmd, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, jcmd.InstallationSpec.Proxy, fromCIDR, toCIDR); err != nil {
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			logrus.Info("Host preflights completed successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().MarkHidden("airgap-bundle")

	cmd.Flags().StringVarP(&license, "license", "l", "", "Path to the license file")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts.")

	return cmd
}

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join token.
type JoinCommandResponse struct {
	K0sJoinCommand         string                     `json:"k0sJoinCommand"`
	K0sToken               string                     `json:"k0sToken"`
	ClusterID              uuid.UUID                  `json:"clusterID"`
	EmbeddedClusterVersion string                     `json:"embeddedClusterVersion"`
	AirgapRegistryAddress  string                     `json:"airgapRegistryAddress"`
	InstallationSpec       ecv1beta1.InstallationSpec `json:"installationSpec,omitempty"`
}

// extractK0sConfigOverridePatch parses the provided override and returns a dig.Mapping that
// can be then applied on top a k0s configuration file to set both `api` and `storage` spec
// fields. All other fields in the override are ignored.
func (j JoinCommandResponse) extractK0sConfigOverridePatch(data []byte) (dig.Mapping, error) {
	config := dig.Mapping{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded config: %w", err)
	}
	result := dig.Mapping{}
	if api := config.DigMapping("config", "spec", "api"); len(api) > 0 {
		result.DigMapping("config", "spec")["api"] = api
	}
	if storage := config.DigMapping("config", "spec", "storage"); len(storage) > 0 {
		result.DigMapping("config", "spec")["storage"] = storage
	}
	return result, nil
}

// EndUserOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the EndUserK0sConfigOverrides field.
func (j JoinCommandResponse) EndUserOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.InstallationSpec.EndUserK0sConfigOverrides))
}

// EmbeddedOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the K0sUnsupportedOverrides field.
func (j JoinCommandResponse) EmbeddedOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.InstallationSpec.Config.UnsupportedOverrides.K0s))
}

// getJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func getJoinToken(ctx context.Context, baseURL, shortToken string) (*JoinCommandResponse, error) {
	url := fmt.Sprintf("https://%s/api/v1/embedded-cluster/join?token=%s", baseURL, shortToken)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	// this will generally be a self-signed certificate created by kurl-proxy
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := insecureClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get join token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var command JoinCommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&command); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}
	return &command, nil
}
