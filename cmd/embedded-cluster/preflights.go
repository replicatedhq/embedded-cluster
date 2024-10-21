package main

import (
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// installRunPreflightsCommand runs install host preflights.
func installRunPreflightsCommand() *cli.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	return &cli.Command{
		Name:   "run-preflights",
		Hidden: true,
		Usage:  "Run install host preflights",
		Flags: withProxyFlags(withSubnetCIDRFlags(
			[]cli.Flag{
				&cli.StringFlag{
					Name:   "airgap-bundle",
					Usage:  "Path to the air gap bundle. If set, the installation will complete without internet access.",
					Hidden: true,
				},
				&cli.StringFlag{
					Name:    "license",
					Aliases: []string{"l"},
					Usage:   "Path to the license file.",
					Hidden:  false,
				},
				&cli.BoolFlag{
					Name:  "no-prompt",
					Usage: "Disable interactive prompts.",
					Value: false,
				},
				getInstallDataDirFlag(runtimeConfig),
				getAdminConsolePortFlag(runtimeConfig),
				getLocalArtifactMirrorPortFlag(runtimeConfig),
			},
		)),
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("run-preflights command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

			var err error
			proxy, err := getProxySpecFromFlags(c)
			if err != nil {
				return fmt.Errorf("unable to get proxy spec from flags: %w", err)
			}

			proxy, err = includeLocalIPInNoProxy(c, proxy)
			if err != nil {
				return err
			}
			setProxyEnv(proxy)

			license, err := getLicenseFromFilepath(c.String("license"))
			if err != nil {
				return err
			}

			isAirgap := c.String("airgap-bundle") != ""

			logrus.Debugf("materializing binaries")
			if err := materializeFiles(c, provider); err != nil {
				return err
			}

			applier, err := getAddonsApplier(c, runtimeConfig, "", proxy)
			if err != nil {
				return err
			}

			logrus.Debugf("running host preflights")
			var replicatedAPIURL, proxyRegistryURL string
			if license != nil {
				replicatedAPIURL = license.Spec.Endpoint
				proxyRegistryURL = fmt.Sprintf("https://%s", defaults.ProxyRegistryAddress)
			}
			if err := RunHostPreflights(c, provider, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, proxy); err != nil {
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			logrus.Info("Host preflights completed successfully")

			return nil
		},
	}
}

// joinRunPreflightsCommand runs install host preflights.
var joinRunPreflightsCommand = &cli.Command{
	Name:      "run-preflights",
	Hidden:    true,
	Usage:     "Run join host preflights",
	ArgsUsage: "<url> <token>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   "airgap-bundle",
			Usage:  "Path to the air gap bundle. If set, the installation will complete without internet access.",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:  "network-interface",
			Usage: "The network interface to use for the cluster",
			Value: "",
		},
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Disable interactive prompts.",
			Value: false,
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("run-preflights command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() != 2 {
			return fmt.Errorf("usage: %s join preflights <url> <token>", binName)
		}

		logrus.Debugf("fetching join token remotely")
		jcmd, err := getJoinToken(c.Context, c.Args().Get(0), c.Args().Get(1))
		if err != nil {
			return fmt.Errorf("unable to get join token: %w", err)
		}

		provider := defaults.NewProviderFromRuntimeConfig(jcmd.InstallationSpec.RuntimeConfig)
		os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

		defer tryRemoveTmpDirContents(provider)

		// check to make sure the version returned by the join token is the same as the one we are running
		if strings.TrimPrefix(jcmd.EmbeddedClusterVersion, "v") != strings.TrimPrefix(versions.Version, "v") {
			return fmt.Errorf("embedded cluster version mismatch - this binary is version %q, but the cluster is running version %q", versions.Version, jcmd.EmbeddedClusterVersion)
		}

		setProxyEnv(jcmd.InstallationSpec.Proxy)
		proxyOK, localIP, err := checkProxyConfigForLocalIP(jcmd.InstallationSpec.Proxy, c.String("network-interface"))
		if err != nil {
			return fmt.Errorf("failed to check proxy config for local IP: %w", err)
		}
		if !proxyOK {
			return fmt.Errorf("no-proxy config %q does not allow access to local IP %q", jcmd.InstallationSpec.Proxy.NoProxy, localIP)
		}

		isAirgap := c.String("airgap-bundle") != ""

		logrus.Debugf("materializing binaries")
		if err := materializeFiles(c, provider); err != nil {
			return err
		}

		applier, err := getAddonsApplier(c, jcmd.InstallationSpec.RuntimeConfig, "", jcmd.InstallationSpec.Proxy)
		if err != nil {
			return err
		}

		logrus.Debugf("running host preflights")
		replicatedAPIURL := jcmd.InstallationSpec.MetricsBaseURL
		proxyRegistryURL := fmt.Sprintf("https://%s", defaults.ProxyRegistryAddress)
		if err := RunHostPreflights(c, provider, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, jcmd.InstallationSpec.Proxy); err != nil {
			if err == ErrPreflightsHaveFail {
				return ErrNothingElseToAdd
			}
			return err
		}

		logrus.Info("Host preflights completed successfully")

		return nil
	},
}
