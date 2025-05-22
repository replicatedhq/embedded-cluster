package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Controller interface {
	Authenticate(ctx context.Context, password string) (string, error)
	ValidateSessionToken(ctx context.Context, sessionToken string) (bool, error)
}

var _ Controller = &AuthController{}

type AuthController struct {
	password     string
	sessionToken string
}

type AuthControllerOption func(*AuthController)

func NewAuthController(password string, opts ...AuthControllerOption) (*AuthController, error) {
	controller := &AuthController{
		password: password,
	}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.password == "" {
		return nil, fmt.Errorf("password is required")
	}

	return controller, nil
}

func (c *AuthController) Authenticate(ctx context.Context, password string) (string, error) {
	if password != c.password {
		return "", fmt.Errorf("invalid password")
	}

	c.sessionToken = uuid.New().String()

	return c.sessionToken, nil
}

func (c *AuthController) ValidateSessionToken(ctx context.Context, sessionToken string) (bool, error) {
	if sessionToken != c.sessionToken {
		return false, nil
	}

	return true, nil
}
