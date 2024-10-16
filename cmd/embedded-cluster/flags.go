package main

import (
	"fmt"
	"strconv"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	k8snet "k8s.io/utils/net"
)

func getNetworkCIDRFlag(runtimeConfig *ecv1beta1.RuntimeConfigSpec) cli.Flag {
	return &cli.StringFlag{
		Name:  "cidr",
		Usage: "IP Address Range for Pods and Services, allocate a range of at least /16. This will be evenly divided into separate subnets",
		Value: ecv1beta1.DefaultNetworkCIDR,
		Action: func(c *cli.Context, addr string) error {
			if c.IsSet("pod-cidr") || c.IsSet("service-cidr") {
				return fmt.Errorf("--cidr flag can't be used with --pod-cidr or --service-cidr")
			}
			if err := netutils.ValidateCIDR(addr, 16, true); err != nil {
				return err
			}
			logrus.Debugf("Setting network cidr to %q from flag", addr)
			runtimeConfig.NetworkCIDR = addr
			return nil
		},
	}
}

func getAdminConsolePortFlag(runtimeConfig *ecv1beta1.RuntimeConfigSpec) cli.Flag {
	return &cli.StringFlag{
		Name:   "admin-console-port",
		Usage:  "Port on which the Admin Console will be served",
		Value:  strconv.Itoa(ecv1beta1.DefaultAdminConsolePort),
		Hidden: false,
		Action: func(c *cli.Context, s string) error {
			if s == "" {
				return nil
			}
			// TODO: add first class support for service node port range and validate the port
			port, err := k8snet.ParsePort(s, false)
			if err != nil {
				return fmt.Errorf("invalid port: %w", err)
			}
			logrus.Debugf("Setting admin console port to %d from flag", port)
			runtimeConfig.AdminConsole.Port = port
			return nil
		},
	}
}

func getLocalArtifactMirrorPortFlag(runtimeConfig *ecv1beta1.RuntimeConfigSpec) cli.Flag {
	return &cli.StringFlag{
		Name:   "local-artifact-mirror-port",
		Usage:  "Port on which the Local Artifact Mirror will be served",
		Value:  strconv.Itoa(ecv1beta1.DefaultLocalArtifactMirrorPort),
		Hidden: false,
		Action: func(c *cli.Context, s string) error {
			if s == "" {
				return nil
			}
			// TODO: add first class support for service node port range and validate the port does not
			// conflict with this range
			port, err := k8snet.ParsePort(s, false)
			if err != nil {
				return fmt.Errorf("invalid local artifact mirror port: %w", err)
			}
			if s == c.String("admin-console-port") {
				return fmt.Errorf("local artifact mirror port cannot be the same as admin console port")
			}
			logrus.Debugf("Setting local artifact mirror port to %d from flag", port)
			runtimeConfig.LocalArtifactMirror.Port = port
			return nil
		},
	}
}

func getDataDirFlag(runtimeConfig *ecv1beta1.RuntimeConfigSpec) cli.Flag {
	return &cli.StringFlag{
		Name:   "data-dir",
		Usage:  "Path to the data directory",
		Value:  ecv1beta1.DefaultDataDir,
		Hidden: false,
		Action: func(c *cli.Context, s string) error {
			if s == "" {
				return nil
			}
			logrus.Debugf("Setting data dir to %s from flag", s)
			runtimeConfig.DataDir = s
			return nil
		},
	}
}
