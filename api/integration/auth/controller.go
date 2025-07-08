package auth

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
)

var _ auth.Controller = &StaticAuthController{}

type StaticAuthController struct {
	token string
}

// NewStaticAuthController creates a new StaticAuthController with the given token
func NewStaticAuthController(token string) *StaticAuthController {
	return &StaticAuthController{token: token}
}

func (s *StaticAuthController) Authenticate(ctx context.Context, password string) (string, error) {
	return s.token, nil
}

func (s *StaticAuthController) ValidateToken(ctx context.Context, token string) error {
	if token != s.token {
		return fmt.Errorf("invalid token")
	}
	return nil
}
