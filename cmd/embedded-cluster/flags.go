package main

import (
	"fmt"
	"strconv"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	k8snet "k8s.io/utils/net"
)

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
