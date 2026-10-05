package analytics

import (
	"context"
	"fmt"
	"time"
)

type Repository interface {
	Ingest(context.Context, Event, string) error
	Summary(context.Context, string, string, ReportRange, time.Time) (Summary, error)
	Traffic(context.Context, string, string, ReportRange, time.Time) ([]TrafficPoint, error)
	TopPages(context.Context, string, string, int, ReportRange, time.Time) ([]TopPage, error)
	LandingPages(context.Context, string, string, int, ReportRange, time.Time) ([]LandingPage, error)
	UTMReport(context.Context, string, string, ReportRange, time.Time) (UTMReport, error)
	Sources(context.Context, string, string, ReportRange, time.Time) ([]Source, error)
	Countries(context.Context, string, string, ReportRange, time.Time) ([]Country, error)
	Devices(context.Context, string, string, ReportRange, time.Time) (Devices, error)
	Realtime(context.Context, string, string) (Realtime, error)
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
	return NewServiceWithGeo(repository, NullGeoResolver{})
}

// NewServiceWithGeo injects a GeoIP resolver (MaxMind in production).
// Tests that need deterministic geography pass their own resolver.
func NewServiceWithGeo(repository Repository, geo GeoResolver) *Service {
	return &Service{repository: repository, now: time.Now, geo: geo}
}

func (service *Service) Ingest(ctx context.Context, event Event, meta RequestMeta) error {
	if err := event.Validate(service.now().UTC()); err != nil {
		return &ValidationError{err: fmt.Errorf("validate event: %w", err)}
	}
	event.Enrich(meta, service.geo)
	originHost := OriginHost(meta.Origin, meta.Referer)
	return service.repository.Ingest(ctx, event, originHost)
}

const maxTopPages = 100

func (service *Service) Summary(ctx context.Context, workspaceID, trackingID string, rg ReportRange) (Summary, error) {
	if workspaceID == "" {
		return Summary{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return Summary{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return Summary{}, err
	}
	return service.repository.Summary(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) Traffic(ctx context.Context, workspaceID, trackingID string, rg ReportRange) ([]TrafficPoint, error) {
	if workspaceID == "" {
		return []TrafficPoint{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return []TrafficPoint{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return []TrafficPoint{}, err
	}
	return service.repository.Traffic(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) TopPages(ctx context.Context, workspaceID, trackingID string, limit int, rg ReportRange) ([]TopPage, error) {
	if workspaceID == "" {
		return []TopPage{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return []TopPage{}, &ValidationError{err: err}
	}
	if limit < 1 || limit > maxTopPages {
		return []TopPage{}, &ValidationError{err: fmt.Errorf("limit must be between 1 and %d", maxTopPages)}
	}
	if err := rg.Validate(); err != nil {
		return []TopPage{}, err
	}
	return service.repository.TopPages(ctx, workspaceID, trackingID, limit, rg, service.now())
}

func (service *Service) LandingPages(ctx context.Context, workspaceID, trackingID string, limit int, rg ReportRange) ([]LandingPage, error) {
	if workspaceID == "" {
		return []LandingPage{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return []LandingPage{}, &ValidationError{err: err}
	}
	if limit < 1 || limit > maxTopPages {
		return []LandingPage{}, &ValidationError{err: fmt.Errorf("limit must be between 1 and %d", maxTopPages)}
	}
	if err := rg.Validate(); err != nil {
		return []LandingPage{}, err
	}
	return service.repository.LandingPages(ctx, workspaceID, trackingID, limit, rg, service.now())
}

func (service *Service) UTMReport(ctx context.Context, workspaceID, trackingID string, rg ReportRange) (UTMReport, error) {
	if workspaceID == "" {
		return UTMReport{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return UTMReport{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return UTMReport{}, err
	}
	return service.repository.UTMReport(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) Sources(ctx context.Context, workspaceID, trackingID string, rg ReportRange) ([]Source, error) {
	if workspaceID == "" {
		return []Source{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return []Source{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return []Source{}, err
	}
	return service.repository.Sources(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) Countries(ctx context.Context, workspaceID, trackingID string, rg ReportRange) ([]Country, error) {
	if workspaceID == "" {
		return []Country{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return []Country{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return []Country{}, err
	}
	return service.repository.Countries(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) Devices(ctx context.Context, workspaceID, trackingID string, rg ReportRange) (Devices, error) {
	if workspaceID == "" {
		return Devices{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return Devices{}, &ValidationError{err: err}
	}
	if err := rg.Validate(); err != nil {
		return Devices{}, err
	}
	return service.repository.Devices(ctx, workspaceID, trackingID, rg, service.now())
}

func (service *Service) Realtime(ctx context.Context, workspaceID, trackingID string) (Realtime, error) {
	if workspaceID == "" {
		return Realtime{}, &ValidationError{err: fmt.Errorf("authentication required")}
	}
	if err := validateTrackingFilter(trackingID); err != nil {
		return Realtime{}, &ValidationError{err: err}
	}
	return service.repository.Realtime(ctx, workspaceID, trackingID)
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
