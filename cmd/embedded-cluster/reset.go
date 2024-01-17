package main

import (
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

func getEtcdMemberAddress(hostname string) (string, error) {
	k0s := defaults.K0sBinaryPath()
	out, err := exec.Command(k0s, "etcd", "member-list").Output()
	if err != nil {
		fmt.Println(err)
		return "", nil
	}
	members := etcdMembers{}
	json.Unmarshal(out, &members)
	if _, ok := members.Members[hostname]; ok {
		url, err := url.Parse(members.Members[hostname])
		if err != nil {
			return "", err
		}
		return strings.SplitN(url.Host, ":", 2)[0], nil
	} else {
		return "", errors.New("unable to find host in etcd members list")
	}
}

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Reset the node this command is run from",
	Action: func(c *cli.Context) error {
		fmt.Println("gathering facts...")
		k0s := defaults.K0sBinaryPath()
		binName := defaults.BinaryName()
		adminConfig, err := exec.Command(k0s, "kubeconfig", "admin").Output()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		hostname, err := os.Hostname()
		if err != nil {
			return nil
		}

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

		kcli, err := client.New(cfg, client.Options{})
		autopilot.AddToScheme(kcli.Scheme())
		if err != nil {
			fmt.Println("couldn't create k8s config: ", err)
			return err
		}

		node := corev1.Node{}
		err = kcli.Get(c.Context, client.ObjectKey{Name: hostname}, &node)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		isControlPlane := false
		labels := node.GetLabels()
		if labels["node-role.kubernetes.io/control-plane"] == "true" {
			isControlPlane = true
		}

		// drain node
		drainArgList := []string{
			"kubectl",
			"drain",
			"--ignore-daemonsets",
			"--delete-emptydir-data",
			hostname,
		}
		fmt.Println("draining node...")
		_, err = exec.Command(k0s, drainArgList...).Output()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// remove node from cluster
		fmt.Println("removing node from cluster...")
		err = kcli.Delete(c.Context, &node)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// controller pre-reset
		if isControlPlane {

			controlNode := autopilot.ControlNode{}
			err = kcli.Get(c.Context, client.ObjectKey{Name: hostname}, &controlNode)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			err := kcli.Delete(c.Context, &controlNode)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			etcdPeerAddress, err := getEtcdMemberAddress(hostname)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			if etcdPeerAddress != "" {
				fmt.Println("leaving etcd cluster...")
				stdout, err := exec.Command(k0s, "etcd", "leave", "--peer-address", etcdPeerAddress).Output()
				if err != nil {
					fmt.Println(err, string(stdout))
					return nil
				}
			}

		}

		// reset local node
		fmt.Printf("stopping %s...\n", binName)
		stdout, err := exec.Command(k0s, "stop").Output()
		if err != nil {
			fmt.Println(err, string(stdout))
			return nil
		}

		fmt.Printf("resetting %s...\n", binName)
		stdout, err = exec.Command(k0s, "reset").Output()
		if err != nil {
			fmt.Println(err, string(stdout))
			return nil
		}
		fmt.Println(string(stdout))

		return nil
	},
}
