package call

import (
	"fmt"

	lk "github.com/team-dang/aiseebackend/pkg/livekit"
)

type Service struct {
	lk *lk.Client
}

func NewService(lkClient *lk.Client) *Service {
	return &Service{lk: lkClient}
}

func (s *Service) GetUserToken(userID, userName string) (string, error) {
	roomName := fmt.Sprintf("user-%s", userID)
	return s.lk.CreateToken(roomName, userID, userName)
}
