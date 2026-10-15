package service

import (
	"context"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// Analytics top-N defaults/limits. They mirror the HTTP-level contract of the
// analytics endpoints.
const (
	DefaultAnalyticsLimit = 10
	MaxAnalyticsLimit     = 100
)

// TargetData is the read port over lab_target consumed by the HTTP layer. It
// exposes the same entity reads as SourceData (lab_target mirrors lab_source)
// plus the analytics aggregates.
type TargetData interface {
	SourceData

	// Summary returns entity counts, revenue totals and the orders-by-status
	// breakdown.
	Summary(ctx context.Context) (models.AnalyticsSummary, error)
	// TopProducts returns the top-selling products, limited to limit rows.
	TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error)
	// TopCustomers returns the highest-spending customers, limited to limit
	// rows.
	TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error)
}

// TargetDataService implements TargetData by delegating entity reads to a
// SourceData built over the lab_target repositories and analytics to an
// AnalyticsRepository.
type TargetDataService struct {
	SourceData
	analytics repository.AnalyticsRepository
}

// NewTargetDataService builds a TargetDataService from a lab_target entity
// read port and the analytics repository.
func NewTargetDataService(data SourceData, analytics repository.AnalyticsRepository) *TargetDataService {
	return &TargetDataService{SourceData: data, analytics: analytics}
}

// NormalizeAnalyticsLimit clamps a requested top-N limit to the contract:
// omitted/invalid values fall back to DefaultAnalyticsLimit and values above
// MaxAnalyticsLimit are capped.
func NormalizeAnalyticsLimit(limit int) int {
	switch {
	case limit < 1:
		return DefaultAnalyticsLimit
	case limit > MaxAnalyticsLimit:
		return MaxAnalyticsLimit
	default:
		return limit
	}
}

// Summary returns the lab_target aggregate snapshot.
func (s *TargetDataService) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	summary, err := s.analytics.Summary(ctx)
	if err != nil {
		return models.AnalyticsSummary{}, wrapErr("analytics summary", err)
	}
	return summary, nil
}

// TopProducts returns the top-selling products, limited to limit rows.
func (s *TargetDataService) TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error) {
	rows, err := s.analytics.TopProducts(ctx, NormalizeAnalyticsLimit(limit))
	if err != nil {
		return nil, wrapErr("analytics top products", err)
	}
	return rows, nil
}

// TopCustomers returns the highest-spending customers, limited to limit rows.
func (s *TargetDataService) TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error) {
	rows, err := s.analytics.TopCustomers(ctx, NormalizeAnalyticsLimit(limit))
	if err != nil {
		return nil, wrapErr("analytics top customers", err)
	}
	return rows, nil
}
