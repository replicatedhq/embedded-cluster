package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidPassword = errors.New("invalid password")

type Controller interface {
	Authenticate(ctx context.Context, password string) (string, error)
	ValidateToken(ctx context.Context, token string) error
}

var _ Controller = (*AuthController)(nil)

type AuthController struct {
	passwordHash []byte
}

type AuthControllerOption func(*AuthController)

func NewAuthController(passwordHash []byte, opts ...AuthControllerOption) (*AuthController, error) {
	if len(passwordHash) == 0 {
		return nil, fmt.Errorf("password hash is required")
	}

	controller := &AuthController{
		passwordHash: passwordHash,
	}

	for _, opt := range opts {
		opt(controller)
	}

	return controller, nil
}

func (c *AuthController) Authenticate(ctx context.Context, password string) (string, error) {
	err := bcrypt.CompareHashAndPassword(c.passwordHash, []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return "", ErrInvalidPassword
		}
		return "", fmt.Errorf("failed to verify password: %w", err)
	}

	token, err := getToken("admin")
	if err != nil {
		return "", fmt.Errorf("failed to create session token: %w", err)
	}

	return token, nil
}

func (c *AuthController) ValidateToken(ctx context.Context, token string) error {
	_, err := validateToken(token)
	return err
}
