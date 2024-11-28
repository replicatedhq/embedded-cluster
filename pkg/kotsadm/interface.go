package kotsadm

import (
	"context"
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
	GetJoinToken(ctx context.Context, baseURL, shortToken string) (*JoinCommandResponse, error)
}

// Convenience functions

// GetJoinToken is a helper function that issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func GetJoinToken(ctx context.Context, baseURL, shortToken string) (*JoinCommandResponse, error) {
	return _kotsadm.GetJoinToken(ctx, baseURL, shortToken)
}
