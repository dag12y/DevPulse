package analytics

import (
	"context"
	"fmt"
	"time"
)

type Repository interface {
	Ingest(context.Context, Event) error
}

type Service struct {
	repository Repository
	now        func() time.Time
}

type ValidationError struct {
	err error
}

func (err *ValidationError) Error() string {
	return err.err.Error()
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (service *Service) Ingest(ctx context.Context, event Event) error {
	if err := event.Validate(service.now().UTC()); err != nil {
		return &ValidationError{err: fmt.Errorf("validate event: %w", err)}
	}
	return service.repository.Ingest(ctx, event)
}
