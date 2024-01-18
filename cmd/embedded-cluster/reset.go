package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	autopilot "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

type etcdMembers struct {
	Members map[string]string `json:"members"`
}

type hostInfo struct {
	Hostname          string
	Kclient           client.Client
	Node              corev1.Node
	ControlNode       autopilot.ControlNode
	EtcdMemberAddress string
}

var (
	k0s     = defaults.K0sBinaryPath()
	binName = defaults.BinaryName()
)

// getEtcdMemberAddress uses k0s to obtain the etcd member address for the given hostname
func (h *hostInfo) getEtcdMemberAddress() error {
	k0s := defaults.K0sBinaryPath()
	out, err := exec.Command(k0s, "etcd", "member-list").Output()
	if err != nil {
		return err
	}
	members := etcdMembers{}
	json.Unmarshal(out, &members)
	if _, ok := members.Members[h.Hostname]; ok {
		url, err := url.Parse(members.Members[h.Hostname])
		if err != nil {
			return err
		}
		h.EtcdMemberAddress = strings.SplitN(url.Host, ":", 2)[0]
		return nil
	} else {
		return errors.New("unable to find host in etcd members list")
	}
}

// drainNode uses k0s to initiate a node drain
func (h *hostInfo) drainNode() error {
	drainArgList := []string{
		"kubectl",
		"drain",
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		h.Hostname,
	}
	fmt.Println("draining node...")
	out, err := exec.Command(k0s, drainArgList...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not drain node: %w, %s", err, out)
	}
	return nil
}

// configureKubernetesClient sets up a client to use for kubernetes api calls
func (h *hostInfo) configureKubernetesClient() error {
	adminConfig, err := exec.Command(k0s, "kubeconfig", "admin").Output()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	tempKubeConfig, err := os.CreateTemp("", "*")
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer os.Remove(tempKubeConfig.Name())
	defer tempKubeConfig.Close()
	err = os.WriteFile(tempKubeConfig.Name(), adminConfig, os.ModeAppend)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	os.Setenv("KUBECONFIG", tempKubeConfig.Name())
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Println("couldn't get k8s config: ", err)
		return err
	}
	h.Kclient, err = client.New(cfg, client.Options{})
	autopilot.AddToScheme(h.Kclient.Scheme())
	if err != nil {
		fmt.Println("couldn't create k8s config: ", err)
		return err
	}
	return nil
}

// getHostName fetches the hostname for the node
func (h *hostInfo) getHostName() error {
	hostname, err := os.Hostname()
	if err != nil {
		return nil
	}
	h.Hostname = hostname
	return nil
}

// detectControlPlane attempts to determine if the node is a controlplane node
func (h *hostInfo) isControlPlane() (bool, error) {
	labels := h.Node.GetLabels()
	if labels["node-role.kubernetes.io/control-plane"] == "true" {
		return true, nil
	}
	return false, nil
}

// getNodeObject fetches the node object from the k8s api server
func (h *hostInfo) getNodeObject(ctx context.Context) error {
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.Node)
	if err != nil {
		return err
	}
	return nil
}

// getControlNodeObject fetches the controlNode object from the k8s api server
func (h *hostInfo) getControlNodeObject(ctx context.Context) error {
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.ControlNode)
	if err != nil {
		return err
	}
	return nil
}

// leaveEtcdcluster uses k0s to attempt to leave the etcd cluster
func (h *hostInfo) leaveEtcdcluster() error {
	if h.EtcdMemberAddress == "" {
		return errors.New("host has no etcd member address")
	}
	out, err := exec.Command(k0s, "etcd", "leave", "--peer-address", h.EtcdMemberAddress).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to leave etcd cluster: %w, %s", err, string(out))
	}
	return nil
}

// stopK0s attempts to stop the k0s service
func (h *hostInfo) stopK0s() error {
	out, err := exec.Command(k0s, "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not stop k0s service: %w, %s", err, string(out))
	}
	return nil
}

// resetK0s attempts to reset k0s
func (h *hostInfo) resetK0s() error {
	out, err := exec.Command(k0s, "reset").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not reset k0s: %w, %s", err, string(out))
	}
	return nil
}

// newHostInfo returns a populated hostInfo struct
func newHostInfo(ctx context.Context) (hostInfo, error) {
	currentHost := hostInfo{}
	// populate hostname
	err := currentHost.getHostName()
	if err != nil {
		return currentHost, err
	}
	// set up kube client
	err = currentHost.configureKubernetesClient()
	if err != nil {
		return currentHost, err
	}
	// fetch node object
	err = currentHost.getNodeObject(ctx)
	if err != nil {
		return currentHost, err
	}
	// control plane only stuff
	if ok, err := currentHost.isControlPlane(); err != nil && ok {
		// fetch controlNode
		err := currentHost.getControlNodeObject(ctx)
		if err != nil {
			return currentHost, err
		}
		// fetch etcd member address
		err = currentHost.getEtcdMemberAddress()
		if err != nil {
			return currentHost, err
		}
	} else if err != nil {
		return currentHost, err
	}
	return currentHost, nil
}

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Reset the node this command is run from",
	Action: func(c *cli.Context) error {

		fmt.Println("gathering facts...")
		// populate options struct with host information
		currentHost, err := newHostInfo(c.Context)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// drain node
		err = currentHost.drainNode()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// remove node from cluster
		fmt.Println("removing node from cluster...")
		err = currentHost.Kclient.Delete(c.Context, &currentHost.Node)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// controller pre-reset
		if ok, err := currentHost.isControlPlane(); err != nil && ok {
			// delete controlNode object from cluster
			err := currentHost.Kclient.Delete(c.Context, &currentHost.ControlNode)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			// try and leave etcd cluster
			fmt.Println("leaving etcd cluster...")
			err = currentHost.leaveEtcdcluster()
			if err != nil {
				fmt.Println(err)
				return nil
			}
		} else if err != nil {
			fmt.Println(err)
			return nil
		}

		// stop k0s
		fmt.Printf("stopping %s...\n", binName)
		err = currentHost.stopK0s()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// reset local node
		fmt.Printf("resetting %s...\n", binName)
		err = currentHost.resetK0s()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		return nil
	},
}
