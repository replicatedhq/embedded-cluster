package k0s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

const (
	k0sBinPath = "/usr/local/bin/k0s"
)

var (
	_ ClientInterface = (*Client)(nil)
)

type Client struct {
}

// GetStatus calls the k0s status command and returns information about system init, PID, k0s role,
// kubeconfig and similar.
func (k *Client) GetStatus(ctx context.Context) (*K0sStatus, error) {
	if _, err := os.Stat(k0sBinPath); err != nil {
		return nil, fmt.Errorf("%s does not seem to be installed on this node", defaults.BinaryName())
	}

	// get k0s status json
	out, err := exec.CommandContext(ctx, k0sBinPath, "status", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var status K0sStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
