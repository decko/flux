package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/jwtutil"
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

// Register creates a new user with the given email and password.
// It hashes the password using bcrypt, assigns the default "user" role,
// and persists the user via the repository.
// Returns ErrDuplicateEmail if the email already exists.
// Returns validation errors if any required field is empty or if the
// email format is invalid.
func (s *AuthService) Register(ctx context.Context, email, password string) (model.User, error) {
	if email == "" {
		return model.User{}, fmt.Errorf("email is required")
	}
	if password == "" {
		return model.User{}, fmt.Errorf("password is required")
	}
	if err := validateEmail(email); err != nil {
		return model.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hashing password: %w", err)
	}

	user := model.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         "user",
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

// validateEmail performs basic email format validation.
// It checks for the presence of "@" and a non-empty domain part.
func validateEmail(email string) error {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid email format")
	}
	return nil
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
// an extended expiry. The user is re-fetched from the database to ensure
// the account is still active. Returns an error if the token is invalid
// or expired.
func (s *AuthService) RefreshToken(ctx context.Context, tokenString string) (string, error) {
	claims, err := jwtutil.ValidateJWTToken(tokenString, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	userID, _ := claims.GetSubject()

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("refresh token: user not found")
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", fmt.Errorf("refreshing token: %w", err)
	}

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
