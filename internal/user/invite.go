package user

import (
	"crypto/rand"
	"fmt"
)

func GenerateInviteCode() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("generate invite code: " + err.Error())
	}
	value := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return fmt.Sprintf("%06d", value%1000000)
}
