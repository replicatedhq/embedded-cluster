package kotsadm

import (
	"context"
	"io"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
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
	GetJoinToken(ctx context.Context, kotsAPIAddress, shortToken string) (*join.JoinCommandResponse, error)
	GetK0sImagesFile(ctx context.Context, kotsAPIAddress string) (io.ReadCloser, error)
}

// Convenience functions

// GetJoinToken is a helper function that issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func GetJoinToken(ctx context.Context, kotsAPIAddress, shortToken string) (*join.JoinCommandResponse, error) {
	return _kotsadm.GetJoinToken(ctx, kotsAPIAddress, shortToken)
}

// GetK0sImagesFile is a helper function that fetches the k0s images file from the KOTS API.
// caller is responsible for closing the response body.
func GetK0sImagesFile(ctx context.Context, kotsAPIAddress string) (io.ReadCloser, error) {
	return _kotsadm.GetK0sImagesFile(ctx, kotsAPIAddress)
}
