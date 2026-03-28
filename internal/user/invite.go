package user

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateInviteCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
