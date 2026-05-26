package supporter

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

const (
	coinsPerAd          = 100
	maxAdsPerDay        = 10
	communityRedeemRate = 100
)

var (
	ErrDailyAdLimitReached = errors.New("đã đạt giới hạn quảng cáo hôm nay (10/ngày)")
	ErrInsufficientBalance = errors.New("số dư không đủ")
	ErrInvalidRecipientID  = errors.New("mã người nhận không hợp lệ")
)

type Service struct {
	queries *dbgen.Queries
	db      *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(db),
		db:      db,
	}
}

type PersonalStats struct {
	SupportedPeopleCount int32 `json:"supported_people_count"`
	TotalAITurnsGifted   int32 `json:"total_ai_turns_gifted"`
	AdsWatchedToday      int32 `json:"ads_watched_today"`
	CoinBalance          int32 `json:"coin_balance"`
	TotalEarned          int32 `json:"total_earned"`
	TotalRedeemed        int32 `json:"total_redeemed"`
}

type CommunityStats struct {
	TotalCaredUsers    int32                `json:"total_cared_users"`
	TotalAITurnsGifted int32                `json:"total_ai_turns_gifted"`
	ActiveSupporters   int32                `json:"active_supporters"`
	Supporters         []CommunitySupporter `json:"supporters"`
}

type CommunitySupporter struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type OverviewResponse struct {
	Personal  PersonalStats  `json:"personal"`
	Community CommunityStats `json:"community"`
}

type WatchAdResponse struct {
	CoinsEarned     int32 `json:"coins_earned"`
	NewBalance      int32 `json:"new_balance"`
	AdsWatchedToday int32 `json:"ads_watched_today"`
}

type RedeemRequest struct {
	Amount          int32  `json:"amount"`
	RecipientUserID string `json:"recipient_user_id,omitempty"`
}

type RedeemResponse struct {
	CoinsRedeemed  int32 `json:"coins_redeemed"`
	NewBalance     int32 `json:"new_balance"`
	AITurnsGranted int32 `json:"ai_turns_granted"`
}

func (s *Service) GetOverview(ctx context.Context, userID pgtype.UUID) (*OverviewResponse, error) {
	stats, err := s.queries.GetPersonalSupporterStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	supportedCount, err := s.queries.CountSupportedPeople(ctx, userID)
	if err != nil {
		return nil, err
	}

	totalGifted, err := s.queries.GetTotalAITurnsGiftedByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	community, err := s.queries.GetCommunitySupporterStats(ctx)
	if err != nil {
		return nil, err
	}

	recentSupporters, err := s.queries.ListRecentCommunitySupporters(ctx, 5)
	if err != nil {
		return nil, err
	}
	supporters := make([]CommunitySupporter, 0, len(recentSupporters))
	for _, row := range recentSupporters {
		supporter := CommunitySupporter{
			ID:          row.ID.String(),
			DisplayName: row.DisplayName,
		}
		if row.AvatarUrl.Valid {
			supporter.AvatarURL = row.AvatarUrl.String
		}
		supporters = append(supporters, supporter)
	}

	return &OverviewResponse{
		Personal: PersonalStats{
			SupportedPeopleCount: supportedCount,
			TotalAITurnsGifted:   totalGifted,
			AdsWatchedToday:      stats.AdsWatchedToday,
			CoinBalance:          stats.CoinBalance,
			TotalEarned:          stats.TotalEarned,
			TotalRedeemed:        stats.TotalRedeemed,
		},
		Community: CommunityStats{
			TotalCaredUsers:    community.TotalCaredUsers,
			TotalAITurnsGifted: community.TotalAiTurnsGifted,
			ActiveSupporters:   community.ActiveSupporters,
			Supporters:         supporters,
		},
	}, nil
}

func (s *Service) WatchAd(ctx context.Context, userID pgtype.UUID) (*WatchAdResponse, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	watch, err := q.EnsureAndGetDailyAdWatch(ctx, userID)
	if err != nil {
		return nil, err
	}
	if watch.Count >= maxAdsPerDay {
		return nil, ErrDailyAdLimitReached
	}

	if err := q.IncrementDailyAdWatch(ctx, userID); err != nil {
		return nil, err
	}

	if err := q.UpsertSupporterWallet(ctx, dbgen.UpsertSupporterWalletParams{
		UserID:  userID,
		Balance: coinsPerAd,
	}); err != nil {
		return nil, err
	}

	if err := q.InsertSupporterTransaction(ctx, dbgen.InsertSupporterTransactionParams{
		UserID: userID,
		Type:   "watch_ad",
		Amount: coinsPerAd,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	wallet, err := s.queries.GetSupporterWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &WatchAdResponse{
		CoinsEarned:     coinsPerAd,
		NewBalance:      wallet.Balance,
		AdsWatchedToday: watch.Count + 1,
	}, nil
}

func (s *Service) Redeem(ctx context.Context, userID pgtype.UUID, req RedeemRequest) (*RedeemResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("số lượng phải lớn hơn 0")
	}

	wallet, err := s.queries.GetSupporterWallet(ctx, userID)
	if err != nil {
		slog.Warn("supporter_redeem_no_wallet", "error", err)
		return nil, errors.New("ví chưa khởi tạo")
	}

	if wallet.Balance < req.Amount {
		return nil, ErrInsufficientBalance
	}

	isCommunityFund := req.RecipientUserID == ""

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	if err := q.DeductSupporterBalance(ctx, dbgen.DeductSupporterBalanceParams{
		UserID:  userID,
		Balance: req.Amount,
	}); err != nil {
		return nil, ErrInsufficientBalance
	}

	var recipientUID pgtype.UUID
	if !isCommunityFund {
		if err := recipientUID.Scan(req.RecipientUserID); err != nil {
			return nil, ErrInvalidRecipientID
		}
	}

	if err := q.InsertSupporterTransaction(ctx, dbgen.InsertSupporterTransactionParams{
		UserID:          userID,
		Type:            "redeem",
		Amount:          req.Amount,
		RecipientUserID: recipientUID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	aiTurns := req.Amount
	if isCommunityFund {
		aiTurns = req.Amount / communityRedeemRate
	}

	newWallet, err := s.queries.GetSupporterWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &RedeemResponse{
		CoinsRedeemed:  req.Amount,
		NewBalance:     newWallet.Balance,
		AITurnsGranted: aiTurns,
	}, nil
}
