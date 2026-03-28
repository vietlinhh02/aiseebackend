package volunteer

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type stubQuerier struct {
	registered bool
	available  bool
}

func (s *stubQuerier) CreateVolunteer(
	_ context.Context, _ dbgen.CreateVolunteerParams,
) (dbgen.Volunteer, error) {
	s.registered = true
	return dbgen.Volunteer{}, nil
}

func (s *stubQuerier) GetVolunteerByUserID(
	_ context.Context, _ pgtype.UUID,
) (dbgen.Volunteer, error) {
	return dbgen.Volunteer{}, nil
}

func (s *stubQuerier) UpdateVolunteerAvailability(
	_ context.Context, p dbgen.UpdateVolunteerAvailabilityParams,
) error {
	s.available = p.Available
	return nil
}

func (s *stubQuerier) GetAvailableVolunteers(
	_ context.Context, _ pgtype.UUID,
) ([]dbgen.GetAvailableVolunteersRow, error) {
	return nil, nil
}

func (s *stubQuerier) CreateVolunteerRating(
	_ context.Context, _ dbgen.CreateVolunteerRatingParams,
) error {
	return nil
}

func (s *stubQuerier) IncrementVolunteerCalls(
	_ context.Context, _ pgtype.UUID,
) error {
	return nil
}

func (s *stubQuerier) RecalcVolunteerRating(
	_ context.Context, _ pgtype.UUID,
) error {
	return nil
}

func (s *stubQuerier) GetUserByID(
	_ context.Context, _ pgtype.UUID,
) (dbgen.User, error) {
	return dbgen.User{}, nil
}

func TestRegister(t *testing.T) {
	store := &stubQuerier{}
	svc := NewService(store, nil, nil, nil)

	err := svc.Register(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		nil,
	)
	if err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if !store.registered {
		t.Fatal("Register: expected registered to be true")
	}
}

func TestSetAvailability(t *testing.T) {
	store := &stubQuerier{}
	svc := NewService(store, nil, nil, nil)

	err := svc.SetAvailability(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		true,
	)
	if err != nil {
		t.Fatalf(
			"SetAvailability: unexpected error: %v", err,
		)
	}
	if !store.available {
		t.Fatal(
			"SetAvailability: expected available to be true",
		)
	}
}
