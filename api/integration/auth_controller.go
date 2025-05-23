package integration

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
)

var _ auth.Controller = &staticAuthController{}

type staticAuthController struct {
	token string
}

func (s *staticAuthController) Authenticate(ctx context.Context, password string) (string, error) {
	return s.token, nil
}

func (s *staticAuthController) ValidateToken(ctx context.Context, token string) error {
	if token != s.token {
		return fmt.Errorf("invalid token")
	}
	return nil
}
