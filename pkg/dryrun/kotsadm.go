package dryrun

import (
	"context"
	"fmt"
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

// SetGetJoinTokenResponse sets the response for the GetJoinToken method, based on the provided baseURL and shortToken.
func (c *Kotsadm) SetGetJoinTokenResponse(baseURL, shortToken string, resp *join.JoinCommandResponse, err error) {
	mockErr := c.setResponse(resp, err, "GetJoinToken", baseURL, shortToken)
	if mockErr != nil {
		panic(mockErr)
	}
}

// GetJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func (c *Kotsadm) GetJoinToken(ctx context.Context, baseURL, shortToken string) (*join.JoinCommandResponse, error) {
	key := strings.Join([]string{"GetJoinToken", baseURL, shortToken}, ":")
	if handler, ok := c.mockHandlers[key]; ok {
		return handler.resp.(*join.JoinCommandResponse), handler.err
	} else {
		return nil, fmt.Errorf("no response set for GetJoinToken, baseURL: %s, shortToken: %s", baseURL, shortToken)
	}
}
