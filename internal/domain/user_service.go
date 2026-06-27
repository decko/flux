package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ValidatePassword checks that a password meets the minimum strength requirements.
// Password must be at least 12 characters long.
func ValidatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("invalid password: must be at least 12 characters")
	}
	return nil
}

// UserService provides business logic for user management operations
// such as listing users, updating roles, and deleting users.
// It enforces security guards: no self-demotion, no self-deletion,
// and prevents the last admin from being demoted or deleted.
type UserService struct {
	userRepo repository.UserRepository
	auditSvc *AuditService
}

// UserServiceOption configures a UserService.
type UserServiceOption func(*UserService)

// WithUserAuditService sets the audit service for recording user management
// events such as role changes and user deletions.
func WithUserAuditService(audit *AuditService) UserServiceOption {
	return func(s *UserService) {
		s.auditSvc = audit
	}
}

// NewUserService creates a new UserService backed by the given repository.
func NewUserService(repo repository.UserRepository, opts ...UserServiceOption) *UserService {
	s := &UserService{userRepo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ListUsers returns all users.
func (s *UserService) ListUsers(ctx context.Context) ([]model.User, error) {
	users, err := s.userRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// UpdateRole changes a user's role. Guards:
//   - newRole must be "admin" or "user"
//   - an actor cannot demote themselves
//   - the last admin cannot be demoted
func (s *UserService) UpdateRole(ctx context.Context, actorID, targetID, newRole string) error {
	if newRole != "admin" && newRole != "user" {
		return fmt.Errorf("invalid role: %s", newRole)
	}
	if actorID == targetID {
		return fmt.Errorf("cannot demote yourself")
	}

	target, err := s.userRepo.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	// Check if we're demoting the last admin.
	if target.Role == "admin" && newRole == "user" {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return fmt.Errorf("count admins: %w", err)
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot demote the last admin")
		}
	}

	target.Role = newRole
	if err := s.userRepo.Update(ctx, target); err != nil {
		return fmt.Errorf("update user role: %w", err)
	}

	if s.auditSvc != nil {
		if err := s.auditSvc.Record(ctx, "user.role_updated", "user", targetID,
			fmt.Sprintf("actor=%q new_role=%q", actorID, newRole)); err != nil {
			return fmt.Errorf("update user role: %w", err)
		}
	}

	return nil
}

// DeleteUser removes a user by ID. Guards:
//   - an actor cannot delete themselves
//   - the last admin cannot be deleted
func (s *UserService) DeleteUser(ctx context.Context, actorID, targetID string) error {
	if actorID == targetID {
		return fmt.Errorf("cannot delete yourself")
	}

	target, err := s.userRepo.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}

	if target.Role == "admin" {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return fmt.Errorf("count admins: %w", err)
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot delete the last admin")
		}
	}

	if err := s.userRepo.Delete(ctx, targetID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if s.auditSvc != nil {
		if err := s.auditSvc.Record(ctx, "user.deleted", "user", targetID,
			fmt.Sprintf("actor=%q", actorID)); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
	}

	return nil
}

// CreateUser creates a new user with the given email, password, and role.
// Password is hashed with bcrypt before storage. Guards:
//   - email must be present
//   - password must pass ValidatePassword (12+ chars)
//   - role must be "admin" or "user"
//   - email must not already exist
//
// PasswordHash is cleared on the returned user. Audits "user.created".
func (s *UserService) CreateUser(ctx context.Context, actorID, email, password, role string) (model.User, error) {
	if email == "" {
		return model.User{}, fmt.Errorf("email is required")
	}
	if err := ValidatePassword(password); err != nil {
		return model.User{}, err
	}
	if role != "admin" && role != "user" {
		return model.User{}, fmt.Errorf("invalid role: %s", role)
	}

	// Check for duplicate email.
	if _, err := s.userRepo.GetByEmail(ctx, email); !errors.Is(err, repository.ErrNotFound) {
		if err == nil {
			return model.User{}, repository.ErrDuplicateEmail
		}
		return model.User{}, fmt.Errorf("check email: %w", err)
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
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	if s.auditSvc != nil {
		if err := s.auditSvc.Record(ctx, "user.created", "user", user.ID,
			fmt.Sprintf("actor=%q email=%q role=%q", actorID, email, role)); err != nil {
			return model.User{}, fmt.Errorf("create user: %w", err)
		}
	}

	user.PasswordHash = ""
	return user, nil
}

// ResetPassword resets the password for the target user. The new password is
// hashed with bcrypt before storage. Guards:
//   - new password must pass ValidatePassword (12+ chars)
//   - target user must exist
//
// PasswordHash is cleared on the returned user. Audits "user.password_reset".
func (s *UserService) ResetPassword(ctx context.Context, actorID, targetID, newPassword string) (model.User, error) {
	if err := ValidatePassword(newPassword); err != nil {
		return model.User{}, err
	}

	target, err := s.userRepo.GetByID(ctx, targetID)
	if err != nil {
		return model.User{}, fmt.Errorf("get target user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hashing password: %w", err)
	}

	target.PasswordHash = string(hash)

	if err := s.userRepo.Update(ctx, target); err != nil {
		return model.User{}, fmt.Errorf("update user: %w", err)
	}

	if s.auditSvc != nil {
		if err := s.auditSvc.Record(ctx, "user.password_reset", "user", targetID,
			fmt.Sprintf("actor=%q", actorID)); err != nil {
			return model.User{}, fmt.Errorf("reset password: %w", err)
		}
	}

	target.PasswordHash = ""
	return target, nil
}
