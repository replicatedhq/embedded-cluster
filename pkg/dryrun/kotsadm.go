package dryrun

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
)

var _ kotsadm.ClientInterface = (*Kotsadm)(nil)

type response struct {
	resp interface{}
	err  error
}

type Kotsadm struct {
	mockHandlers map[string]response
}

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

func (c *Kotsadm) SetGetJoinTokenResponse(baseURL, shortToken string, resp *kotsadm.JoinCommandResponse, err error) {
	mockErr := c.setResponse(resp, err, "GetJoinToken", baseURL, shortToken)
	if mockErr != nil {
		panic(mockErr)
	}
}

// GetJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func (c *Kotsadm) GetJoinToken(ctx context.Context, baseURL, shortToken string) (*kotsadm.JoinCommandResponse, error) {
	key := strings.Join([]string{"GetJoinToken", baseURL, shortToken}, ":")
	fmt.Println(c.mockHandlers)
	if handler, ok := c.mockHandlers[key]; ok {
		return handler.resp.(*kotsadm.JoinCommandResponse), handler.err
	} else {
		return nil, fmt.Errorf("no response set for GetJoinToken, baseURL: %s, shortToken: %s", baseURL, shortToken)
	}
}
