package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLength = 6
	bcryptCost        = 10
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

// NewAuthController creates a new auth controller from a plaintext password
func NewAuthController(password string) (*AuthController, error) {
	if len(password) < minPasswordLength {
		return nil, fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	// Generate bcrypt hash with cost 10 (same as KOTS)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password hash: %w", err)
	}

	return &AuthController{
		passwordHash: hash,
	}, nil
}

// NewAuthControllerFromHash creates a controller with an existing bcrypt hash
func NewAuthControllerFromHash(hash []byte) (*AuthController, error) {
	if len(hash) == 0 {
		return nil, fmt.Errorf("password hash is required")
	}

	return &AuthController{
		passwordHash: hash,
	}, nil
}

func (c *AuthController) Authenticate(ctx context.Context, password string) (string, error) {
	// Compare password with hash using constant-time comparison (same as KOTS)
	err := bcrypt.CompareHashAndPassword(c.passwordHash, []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return "", ErrInvalidPassword
		}
		return "", fmt.Errorf("failed to verify password: %w", err)
	}

	// Generate JWT token
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
