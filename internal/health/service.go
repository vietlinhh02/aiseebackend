package health

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
)

type Service struct {
	queries *dbgen.Queries
}

func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) LogSymptom(
	ctx context.Context,
	userID, symptom, severity, notes string,
) (dbgen.HealthSymptom, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.HealthSymptom{}, err
	}

	if cat, ok := MatchSymptomCategory(symptom); ok {
		symptom = cat
	}

	return s.queries.CreateHealthSymptom(
		ctx, dbgen.CreateHealthSymptomParams{
			UserID:   uid,
			Symptom:  symptom,
			Severity: pgtype.Text{String: severity, Valid: severity != ""},
			Notes:    pgtype.Text{String: notes, Valid: notes != ""},
		},
	)
}

func (s *Service) LogMetric(
	ctx context.Context,
	userID, metricType, value, unit string,
) (dbgen.HealthMetric, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.HealthMetric{}, err
	}

	if mt, defaultUnit, ok := MatchMetricType(metricType); ok {
		metricType = mt
		if unit == "" {
			unit = defaultUnit
		}
	}

	return s.queries.CreateHealthMetric(
		ctx, dbgen.CreateHealthMetricParams{
			UserID:     uid,
			MetricType: metricType,
			Value:      value,
			Unit:       pgtype.Text{String: unit, Valid: unit != ""},
		},
	)
}

func (s *Service) GetSymptoms(
	ctx context.Context,
	userID string, limit, offset int32,
) ([]dbgen.HealthSymptom, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetHealthSymptomsByUser(
		ctx, dbgen.GetHealthSymptomsByUserParams{
			UserID: uid,
			Limit:  limit,
			Offset: offset,
		},
	)
}

func (s *Service) GetMetrics(
	ctx context.Context,
	userID, metricType string,
	limit, offset int32,
) ([]dbgen.HealthMetric, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}

	if metricType != "" {
		if mt, _, ok := MatchMetricType(metricType); ok {
			metricType = mt
		}
		return s.queries.GetHealthMetricsByType(
			ctx, dbgen.GetHealthMetricsByTypeParams{
				UserID:     uid,
				MetricType: metricType,
				Limit:      limit,
				Offset:     offset,
			},
		)
	}

	return s.queries.GetHealthMetricsByUser(
		ctx, dbgen.GetHealthMetricsByUserParams{
			UserID: uid,
			Limit:  limit,
			Offset: offset,
		},
	)
}

func ValidateMetricValue(metricType, value string) error {
	if value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}
