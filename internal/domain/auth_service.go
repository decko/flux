package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// AuthService provides authentication business logic for user registration,
// login, and token refresh. It manages password hashing and JWT token creation.
type AuthService struct {
	userRepo  repository.UserRepository
	jwtSecret []byte
}

// NewAuthService creates a new AuthService with the given user repository
// and JWT signing secret.
func NewAuthService(repo repository.UserRepository, jwtSecret []byte) *AuthService {
	return &AuthService{
		userRepo:  repo,
		jwtSecret: jwtSecret,
	}
}

// Register creates a new user with the given email, password, and role.
// It hashes the password using bcrypt and persists the user via the repository.
// Returns ErrDuplicateEmail if the email already exists.
// Returns validation errors if any required field is empty.
func (s *AuthService) Register(ctx context.Context, email, password, role string) (model.User, error) {
	if email == "" {
		return model.User{}, fmt.Errorf("email is required")
	}
	if password == "" {
		return model.User{}, fmt.Errorf("password is required")
	}
	if role == "" {
		return model.User{}, fmt.Errorf("role is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hashing password: %w", err)
	}

	user := model.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return model.User{}, err
		}
		return model.User{}, fmt.Errorf("register user: %w", err)
	}

	// Clear the password hash before returning to the caller.
	user.PasswordHash = ""
	return user, nil
}

// Login verifies the user's credentials and returns a signed JWT token.
// Returns ErrNotFound if the email does not exist.
// Returns an error if the password is incorrect.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", err
		}
		return "", fmt.Errorf("login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	return token, nil
}

// RefreshToken validates an existing JWT token and returns a new one with
// an extended expiry. Returns an error if the token is invalid or expired.
func (s *AuthService) RefreshToken(ctx context.Context, tokenString string) (string, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	userID, _ := claims.GetSubject()
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("refresh token: user not found")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", fmt.Errorf("refreshing token: %w", err)
	}

	_ = email
	_ = role
	return token, nil
}

// generateToken creates a signed JWT token with standard claims.
// The token expires in 24 hours from creation.
func (s *AuthService) generateToken(user model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"role":  user.Role,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// validateToken parses and validates a JWT token, returning its claims.
func (s *AuthService) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
