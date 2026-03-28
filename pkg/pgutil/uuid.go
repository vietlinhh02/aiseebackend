package pgutil

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func ParsePgUUID(s string) (pgtype.UUID, error) {
	uid, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid: %w", err)
	}
	return pgtype.UUID{Bytes: uid, Valid: true}, nil
}

func PgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	uid := uuid.UUID(u.Bytes)
	return uid.String()
}
