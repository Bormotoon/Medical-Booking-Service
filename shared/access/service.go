// Package access provides access control service implementation.
package access

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// Service implements AccessService interface.
type Service struct {
	blocklist BlocklistRepository
	managers  ManagerRepository
	logger    zerolog.Logger
}

// NewService creates a new access control service.
func NewService(blocklist BlocklistRepository, managers ManagerRepository, logger zerolog.Logger) *Service {
	return &Service{
		blocklist: blocklist,
		managers:  managers,
		logger:    logger.With().Str("component", "access").Logger(),
	}
}

// IsBlocked checks if a user is in the blocklist.
func (s *Service) IsBlocked(ctx context.Context, userID int64) (bool, error) {
	return s.blocklist.IsBlocked(ctx, userID)
}

// GetBlockedUser returns blocked user details.
func (s *Service) GetBlockedUser(ctx context.Context, userID int64) (*BlockedUser, error) {
	return s.blocklist.GetBlockedUser(ctx, userID)
}

// BlockUser adds a user to the blocklist.
func (s *Service) BlockUser(ctx context.Context, userID int64, reason string, blockedBy int64) error {
	// Check if blocker is a manager
	isManager, err := s.managers.IsManager(ctx, blockedBy)
	if err != nil {
		return fmt.Errorf("checking manager status: %w", err)
	}
	if !isManager {
		return fmt.Errorf("user %d is not a manager", blockedBy)
	}

	if err := s.blocklist.BlockUser(ctx, userID, reason, blockedBy); err != nil {
		return err
	}

	s.logger.Info().
		Int64("user_id", userID).
		Int64("blocked_by", blockedBy).
		Str("reason", reason).
		Msg("user blocked")

	return nil
}

// UnblockUser removes a user from the blocklist.
func (s *Service) UnblockUser(ctx context.Context, userID int64) error {
	if err := s.blocklist.UnblockUser(ctx, userID); err != nil {
		return err
	}

	s.logger.Info().
		Int64("user_id", userID).
		Msg("user unblocked")

	return nil
}

// ListBlockedUsers returns all blocked users.
func (s *Service) ListBlockedUsers(ctx context.Context) ([]BlockedUser, error) {
	return s.blocklist.ListBlockedUsers(ctx)
}

// IsManager checks if a user is a manager.
func (s *Service) IsManager(ctx context.Context, userID int64) (bool, error) {
	return s.managers.IsManager(ctx, userID)
}

// GetManager returns manager details.
func (s *Service) GetManager(ctx context.Context, userID int64) (*Manager, error) {
	return s.managers.GetManager(ctx, userID)
}

// AddManager adds a new manager.
func (s *Service) AddManager(ctx context.Context, userID, chatID int64, name string, addedBy int64) error {
	if err := s.managers.AddManager(ctx, userID, chatID, name, addedBy); err != nil {
		return err
	}

	s.logger.Info().
		Int64("user_id", userID).
		Int64("added_by", addedBy).
		Str("name", name).
		Msg("manager added")

	return nil
}

// RemoveManager removes a manager.
func (s *Service) RemoveManager(ctx context.Context, userID int64) error {
	if err := s.managers.RemoveManager(ctx, userID); err != nil {
		return err
	}

	s.logger.Info().
		Int64("user_id", userID).
		Msg("manager removed")

	return nil
}

// ListManagers returns all managers.
func (s *Service) ListManagers(ctx context.Context) ([]Manager, error) {
	return s.managers.ListManagers(ctx)
}

// GetManagerChatIDs returns all manager chat IDs.
func (s *Service) GetManagerChatIDs(ctx context.Context) ([]int64, error) {
	return s.managers.GetManagerChatIDs(ctx)
}

// CanAccess checks if user can access the bot.
// Returns false with reason if user is blocked.
func (s *Service) CanAccess(ctx context.Context, userID int64) (bool, string, error) {
	blocked, err := s.blocklist.GetBlockedUser(ctx, userID)
	if err != nil {
		return true, "", nil // Not blocked
	}
	if blocked != nil {
		reason := "Доступ заблокирован."
		if blocked.Reason != "" {
			reason = fmt.Sprintf("Доступ заблокирован: %s", blocked.Reason)
		}
		return false, reason, nil
	}
	return true, "", nil
}

// CanManage checks if user has manager permissions.
func (s *Service) CanManage(ctx context.Context, userID int64) (bool, error) {
	return s.managers.IsManager(ctx, userID)
}

// Middleware provides a function to check access before handling commands.
func (s *Service) Middleware(ctx context.Context, userID int64) error {
	canAccess, reason, err := s.CanAccess(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking access: %w", err)
	}
	if !canAccess {
		return &AccessDeniedError{Reason: reason}
	}
	return nil
}

// ManagerMiddleware provides a function to check manager permissions.
func (s *Service) ManagerMiddleware(ctx context.Context, userID int64) error {
	canManage, err := s.CanManage(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking manager status: %w", err)
	}
	if !canManage {
		return &AccessDeniedError{Reason: "Эта команда доступна только менеджерам."}
	}
	return nil
}

// AccessDeniedError is returned when user access is denied.
type AccessDeniedError struct {
	Reason string
}

func (e *AccessDeniedError) Error() string {
	return e.Reason
}

// IsAccessDenied checks if error is access denied.
func IsAccessDenied(err error) bool {
	_, ok := err.(*AccessDeniedError)
	return ok
}
