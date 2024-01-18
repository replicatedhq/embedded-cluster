package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	autopilot "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

type etcdMembers struct {
	Members map[string]string `json:"members"`
}

type hostInfo struct {
	Hostname    string
	Kclient     client.Client
	Node        corev1.Node
	ControlNode autopilot.ControlNode
}

var (
	binName = defaults.BinaryName()
	k0s     = defaults.K0sBinaryPath()
)

// drainNode uses k0s to initiate a node drain
func (h *hostInfo) drainNode() error {
	os.Unsetenv("KUBECONFIG")
	drainArgList := []string{
		"kubectl",
		"drain",
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		h.Hostname,
	}
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
		return err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(adminConfig)
	if err != nil {
		return err
	}
	h.Kclient, err = client.New(restConfig, client.Options{})
	autopilot.AddToScheme(h.Kclient.Scheme())
	if err != nil {
		return fmt.Errorf("couldn't create k8s config: %w", err)
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

// isControlPlane attempts to determine if the node is a controlplane node
func (h *hostInfo) isControlPlane() bool {
	labels := h.Node.GetLabels()
	return labels["node-role.kubernetes.io/control-plane"] == "true"
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
	out, err := exec.Command(k0s, "etcd", "leave").CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to leave etcd cluster: %w, %s", err, string(out))
	}
	return nil
}

// stopK0s attempts to stop the k0s service
func (h *hostInfo) stopAndResetK0s() error {
	out, err := exec.Command(k0s, "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not stop k0s service: %w, %s", err, string(out))
	}
	out, err = exec.Command(k0s, "reset").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not reset k0s: %w, %s", err, string(out))
	}
  fmt.Println("Node has been reset, please reboot to ensure transient configuration is also reset")
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
	// control plane only stff
	if currentHost.isControlPlane() {
		// fetch controlNode
		err := currentHost.getControlNodeObject(ctx)
		if err != nil {
			return currentHost, err
		}
	}
	return currentHost, nil
}

func checkErrPrompt(err error) bool {
	if err == nil {
		return true
	}
	fmt.Println("-----")
	fmt.Println(err)
	fmt.Println("-----")
	fmt.Println("An error has occured while trying to reset this node.")
	fmt.Println("Continuing may leave the cluster in an unexpected state")
	return prompts.New().Confirm("Do you want to continue anyway?", false)
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

		// determine if this is the only node in the cluster
		// if this is a single node we can skip a lot of steps
		nodeList := corev1.NodeList{}
		currentHost.Kclient.List(c.Context, &nodeList)
		if len(nodeList.Items) == 1 {
			nodeName := nodeList.Items[0].Name
			if nodeName != currentHost.Hostname {
				fmt.Println("detected a single node cluster, but the node's name doesn't match our hostname")
				return nil
			}
			// stop k0s
			fmt.Printf("resetting %s...\n", binName)
			err = currentHost.stopAndResetK0s()
			if !checkErrPrompt(err) {
				return nil
			}
      return nil
		}

		// drain node
		fmt.Println("draining node...")
		err = currentHost.drainNode()
		if !checkErrPrompt(err) {
			return nil
		}

		// remove node from cluster
		fmt.Println("removing node from cluster...")
		err = currentHost.Kclient.Delete(c.Context, &currentHost.Node)
		if !checkErrPrompt(err) {
			return nil
		}

		// controller pre-reset
		if currentHost.isControlPlane() {

			// delete controlNode object from cluster
			fmt.Println("deleting controlNode...")
			err := currentHost.Kclient.Delete(c.Context, &currentHost.ControlNode)
			if !checkErrPrompt(err) {
				return nil
			}

			// try and leave etcd cluster
			fmt.Println("leaving etcd cluster...")
			err = currentHost.leaveEtcdcluster()
			if !checkErrPrompt(err) {
				return nil
			}

		} else if err != nil {
			fmt.Println(err)
			return nil
		}

		// reset
		fmt.Printf("resetting %s...\n", binName)
		err = currentHost.stopAndResetK0s()
		if !checkErrPrompt(err) {
			return nil
		}

		return nil
	},
}
