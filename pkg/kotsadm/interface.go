package kotsadm

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_kotsadm ClientInterface
)

func init() {
	Set(&Client{})
}

func Set(kotsadm ClientInterface) {
	_kotsadm = kotsadm
}

type ClientInterface interface {
	GetJoinCommand(ctx context.Context, cli client.Client, roles []string) (string, error)
	GetJoinToken(ctx context.Context, baseURL, shortToken string) (*join.JoinCommandResponse, error)
}

// Convenience functions

// GetJoinToken is a helper function that issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func GetJoinToken(ctx context.Context, baseURL, shortToken string) (*join.JoinCommandResponse, error) {
	return _kotsadm.GetJoinToken(ctx, baseURL, shortToken)
}

// GetJoinCommand is a helper function that issues a request to the kots api to generate a new join token with the provided roles
func GetJoinCommand(ctx context.Context, cli client.Client, roles []string) (string, error) {
	return _kotsadm.GetJoinCommand(ctx, cli, roles)
}
