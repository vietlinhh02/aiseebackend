package middleware

import (
	"context"
	"errors"
	"fmt"
)

// FamilyChecker verifies whether a family link exists between
// two users. user.Service satisfies this interface.
type FamilyChecker interface {
	CheckFamilyLink(
		ctx context.Context, requesterID, targetID string,
	) (bool, error)
}

// WithUserID injects a user ID into the context.
// Intended for tests and internal plumbing.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// WithRole injects a role into the context.
// Intended for tests and internal plumbing.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// AuthorizeFamily verifies that the requester has the "family"
// role and an active family link to targetUserID.
func AuthorizeFamily(
	ctx context.Context,
	targetUserID string,
	checker FamilyChecker,
) error {
	role := RoleFromContext(ctx)
	if role != "family" {
		return errors.New("forbidden: requires family role")
	}

	requesterID := UserIDFromContext(ctx)
	if requesterID == "" {
		return errors.New("forbidden: missing user ID")
	}

	linked, err := checker.CheckFamilyLink(
		ctx, requesterID, targetUserID,
	)
	if err != nil {
		return fmt.Errorf("family link check failed: %w", err)
	}
	if !linked {
		return errors.New("forbidden: no family link")
	}

	return nil
}
