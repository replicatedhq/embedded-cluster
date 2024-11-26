package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/pkg/socket"
	"github.com/spf13/cobra"
)

func StatusCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: fmt.Sprintf("Get the status of the %s cluster manager", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			return printServerStatus(ctx)
		},
	}

	return cmd
}

func printServerStatus(ctx context.Context) error {
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

	req, err := http.NewRequest("GET", "http://unix/healthz", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}
