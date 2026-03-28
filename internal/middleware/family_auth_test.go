package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

type stubFamilyChecker struct {
	linked bool
	err    error
}

func (s *stubFamilyChecker) CheckFamilyLink(
	ctx context.Context,
	requesterID, targetID string,
) (bool, error) {
	return s.linked, s.err
}

func TestAuthorizeFamily_Success(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "requester-1")
	ctx = middleware.WithRole(ctx, "family")

	checker := &stubFamilyChecker{linked: true}

	if err := middleware.AuthorizeFamily(ctx, "target-1", checker); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestAuthorizeFamily_WrongRole(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "requester-1")
	ctx = middleware.WithRole(ctx, "user")

	checker := &stubFamilyChecker{linked: true}

	err := middleware.AuthorizeFamily(ctx, "target-1", checker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "forbidden: requires family role" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestAuthorizeFamily_NoLink(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "requester-1")
	ctx = middleware.WithRole(ctx, "family")

	checker := &stubFamilyChecker{linked: false}

	err := middleware.AuthorizeFamily(ctx, "target-1", checker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "forbidden: no family link" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestAuthorizeFamily_EmptyUserID(t *testing.T) {
	ctx := middleware.WithRole(context.Background(), "family")

	checker := &stubFamilyChecker{linked: true}

	err := middleware.AuthorizeFamily(ctx, "target-1", checker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "forbidden: missing user ID" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestAuthorizeFamily_CheckerError(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "requester-1")
	ctx = middleware.WithRole(ctx, "family")

	checker := &stubFamilyChecker{
		linked: false,
		err:    errors.New("db connection failed"),
	}

	err := middleware.AuthorizeFamily(ctx, "target-1", checker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "family link check failed: db connection failed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
