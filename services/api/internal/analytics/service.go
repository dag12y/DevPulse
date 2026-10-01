package analytics

import (
	"context"
	"fmt"
	"time"
)

type Repository interface {
	Ingest(context.Context, Event) error
	Summary(context.Context, string) (Summary, error)
	Traffic(context.Context, string, int) ([]TrafficPoint, error)
	TopPages(context.Context, string, int) ([]TopPage, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	geo        GeoResolver
}

type ValidationError struct {
	err error
}

func (err *ValidationError) Error() string {
	return err.err.Error()
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now, geo: NullGeoResolver{}}
}

func (service *Service) Ingest(ctx context.Context, event Event, meta RequestMeta) error {
	if err := event.Validate(service.now().UTC()); err != nil {
		return &ValidationError{err: fmt.Errorf("validate event: %w", err)}
	}
	event.Enrich(meta, service.geo)
	return service.repository.Ingest(ctx, event)
}

const (
	maxTrafficDays = 365
	maxTopPages    = 100
)

func (service *Service) Summary(ctx context.Context, trackingID string) (Summary, error) {
	if err := validateTrackingFilter(trackingID); err != nil {
		return Summary{}, &ValidationError{err: err}
	}
	return service.repository.Summary(ctx, trackingID)
}

func (service *Service) Traffic(ctx context.Context, trackingID string, days int) ([]TrafficPoint, error) {
	if err := validateTrackingFilter(trackingID); err != nil {
		return []TrafficPoint{}, &ValidationError{err: err}
	}
	if days < 1 || days > maxTrafficDays {
		return []TrafficPoint{}, &ValidationError{err: fmt.Errorf("days must be between 1 and %d", maxTrafficDays)}
	}
	return service.repository.Traffic(ctx, trackingID, days)
}

func (service *Service) TopPages(ctx context.Context, trackingID string, limit int) ([]TopPage, error) {
	if err := validateTrackingFilter(trackingID); err != nil {
		return []TopPage{}, &ValidationError{err: err}
	}
	if limit < 1 || limit > maxTopPages {
		return []TopPage{}, &ValidationError{err: fmt.Errorf("limit must be between 1 and %d", maxTopPages)}
	}
	return service.repository.TopPages(ctx, trackingID, limit)
}

func validateTrackingFilter(trackingID string) error {
	if trackingID == "" {
		return nil
	}
	if len(trackingID) > 64 || len(trackingID) < 4 || trackingID[:3] != "dp_" {
		return fmt.Errorf("project_id must be a valid tracking ID")
	}
	return nil
}
