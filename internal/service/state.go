package service

import (
	"context"
	"log"

	"bronivik/internal/domain"
)

type StateService struct {
	stateRepo domain.StateRepository
}

func NewStateService(stateRepo domain.StateRepository) *StateService {
	return &StateService{
		stateRepo: stateRepo,
	}
}

func (s *StateService) GetUserState(ctx context.Context, userID int64) (*domain.UserState, error) {
	state, err := s.stateRepo.GetState(ctx, userID)
	if err != nil {
		log.Printf("Error getting state for user %d: %v", userID, err)
		return nil, err
	}

	return state, nil
}

func (s *StateService) SetUserState(ctx context.Context, userID int64, step string, data map[string]interface{}) error {
	state := &domain.UserState{
		UserID: userID,
		Step:   step,
		Data:   data,
	}
	return s.stateRepo.SetState(ctx, state)
}

func (s *StateService) ClearUserState(ctx context.Context, userID int64) error {
	return s.stateRepo.ClearState(ctx, userID)
}

func (s *StateService) UpdateUserStateData(ctx context.Context, userID int64, key string, value interface{}) error {
	state, err := s.stateRepo.GetState(ctx, userID)
	if err != nil {
		return err
	}
	if state == nil {
		state = &domain.UserState{
			UserID: userID,
			Data:   make(map[string]interface{}),
		}
	}

	if state.Data == nil {
		state.Data = make(map[string]interface{})
	}
	state.Data[key] = value

	return s.stateRepo.SetState(ctx, state)
}
