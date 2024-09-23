package main

import (
	"fmt"
	"strconv"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/urfave/cli/v2"
	k8snet "k8s.io/utils/net"
)

func getAdminColsolePortFlag() cli.Flag {
	return &cli.StringFlag{
		Name:   "admin-console-port",
		Usage:  "Port on which the Admin Console will be served",
		Value:  strconv.Itoa(defaults.AdminConsolePort),
		Hidden: false,
	}
}

func getAdminConsolePortFromFlag(c *cli.Context) (int, error) {
	portStr := c.String("admin-console-port")
	if portStr == "" {
		return defaults.AdminConsolePort, nil
	}
	// TODO: add first class support for service node port range and validate the port
	port, err := k8snet.ParsePort(portStr, false)
	if err != nil {
		return 0, fmt.Errorf("invalid admin console port: %w", err)
	}
	return port, nil
}

func getLocalArtifactMirrorPortFlag() cli.Flag {
	return &cli.StringFlag{
		Name:   "local-artifact-mirror-port",
		Usage:  "Port on which the Local Artifact Mirror will be served",
		Value:  strconv.Itoa(defaults.LocalArtifactMirrorPort),
		Hidden: false,
	}
}

func getLocalArtifactMirrorPortFromFlag(c *cli.Context) (int, error) {
	portStr := c.String("local-artifact-mirror-port")
	if portStr == "" {
		return defaults.LocalArtifactMirrorPort, nil
	}
	// TODO: add first class support for service node port range and validate the port does not
	// conflict with this range
	port, err := k8snet.ParsePort(portStr, false)
	if err != nil {
		return 0, fmt.Errorf("invalid local artifact mirror port: %w", err)
	}
	if portStr == c.String("admin-console-port") {
		return 0, fmt.Errorf("local artifact mirror port cannot be the same as admin console port")
	}
	return port, nil
}
