package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/socket"
	"github.com/spf13/cobra"
)

func SendUpgradeClusterCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "upgrade-cluster",
		Short:  name,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// read the spec from the command arg
			if _, err := os.Stat(args[0]); err != nil {
				return err
			}
			spec, err := ioutil.ReadFile(args[0])
			if err != nil {
				return err
			}

			if err := sendDebugClusterUpgrade(ctx, spec); err != nil {
				return err
			}

			log.Println("Upgrade cluster sent")
			return nil
		},
	}
	return cmd
}

func sendDebugClusterUpgrade(ctx context.Context, spec []byte) error {
	socketPath, err := socket.GetSocketPath()
	if err != nil {
		return err
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	// post
	req, err := http.NewRequest("POST", "http://unix/upgradecluster", bytes.NewBuffer(spec))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// try to read the body
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}
