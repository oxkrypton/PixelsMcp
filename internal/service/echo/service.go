package echo

import "context"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Echo(_ context.Context, message string) (string, error) {
	return message, nil
}
