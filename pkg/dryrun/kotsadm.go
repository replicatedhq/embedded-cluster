package dryrun

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
)

var _ kotsadm.ClientInterface = (*Kotsadm)(nil)

type response struct {
	resp interface{}
	err  error
}

// Kotsadm is a mockable implementation of the kotsadm.ClientInterface.
type Kotsadm struct {
	mockHandlers map[string]response
}

// NewKotsadm returns a new Kotsadm API client, to be used with the dryrun package.
func NewKotsadm() *Kotsadm {
	return &Kotsadm{
		mockHandlers: map[string]response{},
	}
}

func (c *Kotsadm) setResponse(resp interface{}, err error, methodName string, args ...string) error {
	methodExists := false
	t := reflect.TypeOf(c)
	for i := 0; i < t.NumMethod(); i++ {
		if methodExists = t.Method(i).Name == methodName; methodExists {
			break
		}
	}
	if !methodExists {
		return fmt.Errorf("method %s does not exist, cannot mock reponse", methodName)
	}
	key := strings.Join(append([]string{methodName}, args...), ":")
	c.mockHandlers[key] = response{resp: resp, err: err}
	return nil
}

// SetGetJoinTokenResponse sets the response for the GetJoinToken method, based on the provided kotsAPIAddress and shortToken.
func (c *Kotsadm) SetGetJoinTokenResponse(kotsAPIAddress, shortToken string, resp *join.JoinCommandResponse, err error) {
	mockErr := c.setResponse(resp, err, "GetJoinToken", kotsAPIAddress, shortToken)
	if mockErr != nil {
		panic(mockErr)
	}
}

// GetJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func (c *Kotsadm) GetJoinToken(ctx context.Context, kotsAPIAddress, shortToken string) (*join.JoinCommandResponse, error) {
	key := strings.Join([]string{"GetJoinToken", kotsAPIAddress, shortToken}, ":")
	if handler, ok := c.mockHandlers[key]; ok {
		return handler.resp.(*join.JoinCommandResponse), handler.err
	} else {
		return nil, fmt.Errorf("no response set for GetJoinToken, kotsAPIAddress: %s, shortToken: %s", kotsAPIAddress, shortToken)
	}
}

func (c *Kotsadm) SetGetK0sImagesFileResponse(kotsAPIAddress string, resp io.ReadCloser, err error) {
	mockErr := c.setResponse(resp, err, "GetK0sImagesFile", kotsAPIAddress)
	if mockErr != nil {
		panic(mockErr)
	}
}

// GetK0sImagesFile fetches the k0s images file from the KOTS API.
// caller is responsible for closing the response body.
func (c *Kotsadm) GetK0sImagesFile(ctx context.Context, kotsAPIAddress string) (io.ReadCloser, error) {
	key := strings.Join([]string{"GetK0sImagesFile", kotsAPIAddress}, ":")
	if handler, ok := c.mockHandlers[key]; ok {
		return handler.resp.(io.ReadCloser), handler.err
	} else {
		return nil, fmt.Errorf("no response set for GetK0sImagesFile, kotsAPIAddress: %s", kotsAPIAddress)
	}
}
