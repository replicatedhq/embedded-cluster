package auth

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidPassword = errors.New("invalid password")

type Controller interface {
	Authenticate(ctx context.Context, password string) (string, error)
	ValidateToken(ctx context.Context, token string) error
}

var _ Controller = (*AuthController)(nil)

type AuthController struct {
	password string
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
		return "", ErrInvalidPassword
	}

	token, err := getToken("admin")
	if err != nil {
		return "", fmt.Errorf("failed to create session token: %w", err)
	}

	return token, nil
}

func (c *AuthController) ValidateToken(ctx context.Context, token string) error {
	_, err := validateToken(token)
	if err != nil {
		return err
	}

	return nil
}
