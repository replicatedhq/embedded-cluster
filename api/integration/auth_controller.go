package integration

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
)

var _ auth.Controller = &staticAuthController{}

type staticAuthController struct {
	token string
}

func (s *staticAuthController) Authenticate(ctx context.Context, password string) (string, error) {
	return s.token, nil
}

func (s *staticAuthController) ValidateSessionToken(ctx context.Context, sessionToken string) (bool, error) {
	return sessionToken == s.token, nil
}
