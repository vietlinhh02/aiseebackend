package user

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateInviteCode() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("generate invite code: " + err.Error())
	}
	return hex.EncodeToString(b)
}
