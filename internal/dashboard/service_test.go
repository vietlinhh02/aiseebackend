package dashboard

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type stubQueries struct {
	meds     []dbgen.Medication
	logs     []dbgen.MedicationLog
	symptoms []dbgen.HealthSymptom
	metrics  []dbgen.HealthMetric
	audits   []dbgen.AuditLog
	apts     []dbgen.Appointment
	location dbgen.GetUserLocationRow
}

func (s *stubQueries) GetMedicationsByUser(
	_ context.Context, _ pgtype.UUID,
) ([]dbgen.Medication, error) {
	return s.meds, nil
}

func (s *stubQueries) GetMedicationLogsToday(
	_ context.Context, _ pgtype.UUID,
) ([]dbgen.MedicationLog, error) {
	return s.logs, nil
}

func (s *stubQueries) GetRecentSymptoms(
	_ context.Context, _ dbgen.GetRecentSymptomsParams,
) ([]dbgen.HealthSymptom, error) {
	return s.symptoms, nil
}

func (s *stubQueries) GetRecentMetrics(
	_ context.Context, _ dbgen.GetRecentMetricsParams,
) ([]dbgen.HealthMetric, error) {
	return s.metrics, nil
}

func (s *stubQueries) GetAuditLogsByUser(
	_ context.Context, _ dbgen.GetAuditLogsByUserParams,
) ([]dbgen.AuditLog, error) {
	return s.audits, nil
}

func (s *stubQueries) GetUserLocation(
	_ context.Context, _ pgtype.UUID,
) (dbgen.GetUserLocationRow, error) {
	return s.location, nil
}

func (s *stubQueries) GetAppointmentsByUser(
	_ context.Context, _ pgtype.UUID,
) ([]dbgen.Appointment, error) {
	return s.apts, nil
}

func (s *stubQueries) CreateMedication(
	_ context.Context, p dbgen.CreateMedicationParams,
) (dbgen.Medication, error) {
	return dbgen.Medication{
		Name:   p.Name,
		Dosage: p.Dosage,
	}, nil
}

func (s *stubQueries) UpdateMedication(
	_ context.Context, p dbgen.UpdateMedicationParams,
) (dbgen.Medication, error) {
	return dbgen.Medication{
		ID:   p.ID,
		Name: p.Name,
	}, nil
}

func (s *stubQueries) DeactivateMedication(
	_ context.Context, _ dbgen.DeactivateMedicationParams,
) error {
	return nil
}

func (s *stubQueries) CreateAuditLog(
	_ context.Context, _ dbgen.CreateAuditLogParams,
) error {
	return nil
}

func TestHealth_ReturnsMedicationsAndSymptoms(t *testing.T) {
	stub := &stubQueries{
		meds: []dbgen.Medication{
			{
				ID:   pgtype.UUID{Valid: true},
				Name: "Aspirin",
			},
		},
		symptoms: []dbgen.HealthSymptom{
			{Symptom: "headache"},
		},
	}

	svc := NewService(stub)
	resp, err := svc.Health(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
	)
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if len(resp.Medications) != 1 {
		t.Errorf(
			"want 1 medication, got %d", len(resp.Medications),
		)
	}
	if len(resp.RecentSymptoms) != 1 {
		t.Errorf(
			"want 1 symptom, got %d",
			len(resp.RecentSymptoms),
		)
	}
}

func TestHealth_AdherenceFromLogs(t *testing.T) {
	medID := pgtype.UUID{
		Bytes: [16]byte{1},
		Valid: true,
	}
	stub := &stubQueries{
		meds: []dbgen.Medication{
			{ID: medID, Name: "Ibuprofen"},
		},
		logs: []dbgen.MedicationLog{
			{
				MedicationID: medID,
				Skipped:      pgtype.Bool{Bool: false, Valid: true},
			},
		},
	}

	svc := NewService(stub)
	resp, err := svc.Health(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
	)
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if len(resp.Medications) != 1 {
		t.Fatalf(
			"want 1 medication entry, got %d",
			len(resp.Medications),
		)
	}
	if !resp.Medications[0].TakenToday {
		t.Error("want taken_today=true for non-skipped log")
	}
}

func TestMedications_WithStatus(t *testing.T) {
	medID := pgtype.UUID{
		Bytes: [16]byte{1},
		Valid: true,
	}
	stub := &stubQueries{
		meds: []dbgen.Medication{
			{ID: medID, Name: "VitaminC"},
		},
		logs: []dbgen.MedicationLog{
			{
				MedicationID: medID,
				Skipped:      pgtype.Bool{Bool: true, Valid: true},
			},
		},
	}

	svc := NewService(stub)
	resp, err := svc.Medications(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
	)
	if err != nil {
		t.Fatalf("Medications() error: %v", err)
	}
	if len(resp.Medications) != 1 {
		t.Fatalf(
			"want 1 medication, got %d",
			len(resp.Medications),
		)
	}
	if resp.Medications[0].Taken {
		t.Error("want taken=false for skipped log")
	}
	if !resp.Medications[0].Skipped {
		t.Error("want skipped=true for skipped log")
	}
}
