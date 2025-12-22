package service

// import (
// 	"context"
// 	"log"
//
// 	"mega-trainer-go/internal/domain"
// )
//
// type StateService struct {
// 	stateRepo domain.StateRepository
// }
//
// func NewStateService(stateRepo domain.StateRepository) *StateService {
// 	return &StateService{
// 		stateRepo: stateRepo,
// 	}
// }
//
// func (s *StateService) GetUserState(ctx context.Context, userID int64) (*domain.UserState, error) {
// 	log.Printf("Getting state for user %d", userID)
//
// 	state, err := s.stateRepo.GetState(ctx, userID)
// 	if err != nil {
// 		log.Printf("Error getting state for user %d: %v", userID, err)
// 		return nil, err
// 	}
//
// 	if state == nil {
// 		log.Printf("No state found for user %d", userID)
// 	} else {
// 		log.Printf("Found state for user %d: command=%s, step=%d", userID, state.Command, state.Step)
// 	}
//
// 	return state, nil
// }
//
// func (s *StateService) SetUserState(ctx context.Context, userID int64, command string, step int, data map[string]string) error {
// 	state := &domain.UserState{
// 		UserID:  userID,
// 		Command: command,
// 		Step:    step,
// 		Data:    data,
// 	}
// 	return s.stateRepo.SetState(ctx, state)
// }
//
// func (s *StateService) ClearUserState(ctx context.Context, userID int64) error {
// 	return s.stateRepo.ClearState(ctx, userID)
// }
//
// func (s *StateService) UpdateUserStateData(ctx context.Context, userID int64, key, value string) error {
// 	state, err := s.stateRepo.GetState(ctx, userID)
// 	if err != nil {
// 		return err
// 	}
// 	if state == nil {
// 		return nil
// 	}
//
// 	if state.Data == nil {
// 		state.Data = make(map[string]string)
// 	}
// 	state.Data[key] = value
//
// 	return s.stateRepo.SetState(ctx, state)
// }
