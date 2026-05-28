package supporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

const (
	coinsPerAd          = 100
	maxAdsPerDay        = 50
	communityRedeemRate = 100
)

var (
	ErrDailyAdLimitReached = errors.New("đã đạt giới hạn quảng cáo hôm nay (50/ngày)")
	ErrInsufficientBalance   = errors.New("số dư không đủ")
	ErrInvalidRecipientID    = errors.New("mã người nhận không hợp lệ")
	ErrReferralSelfRefer     = errors.New("không thể tự giới thiệu chính mình")
	ErrReferralAlreadyClaimed = errors.New("bạn đã được giới thiệu trước đó")
)

type usageService interface {
	GrantUsageQuota(context.Context, billing.UsageGrantInput) (billing.UsageGrant, error)
}

type Service struct {
	queries  *dbgen.Queries
	db       *pgxpool.Pool
	usageSvc usageService
}

func NewService(db *pgxpool.Pool, usageSvc ...usageService) *Service {
	svc := &Service{
		queries: dbgen.New(db),
		db:      db,
	}
	if len(usageSvc) > 0 && usageSvc[0] != nil {
		svc.usageSvc = usageSvc[0]
	}
	return svc
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
	Activities         []CommunityActivity  `json:"activities"`
}

type CommunitySupporter struct {
	AnonSeed string `json:"anon_seed"`
}

type CommunityActivity struct {
	AnonSeed   string `json:"anon_seed"`
	Type       string `json:"type"`
	Amount     int32  `json:"amount"`
	OccurredAt string `json:"occurred_at"`
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
		supporters = append(supporters, CommunitySupporter{
			AnonSeed: row.AnonSeed,
		})
	}

	recentActivities, err := s.queries.ListRecentCommunityActivities(ctx, 10)
	if err != nil {
		return nil, err
	}
	activities := make([]CommunityActivity, 0, len(recentActivities))
	for _, row := range recentActivities {
		activity := CommunityActivity{
			AnonSeed: row.AnonSeed,
			Type:     row.Type,
			Amount:   row.Amount,
		}
		if row.OccurredAt.Valid {
			activity.OccurredAt = row.OccurredAt.Time.UTC().Format(time.RFC3339)
		}
		activities = append(activities, activity)
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
			Activities:         activities,
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

	if s.usageSvc != nil {
		grantCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if isCommunityFund {
			_ = s.grantToCaredUsers(grantCtx, userID, aiTurns)
		} else {
			_ = s.grantToRecipient(grantCtx, userID, recipientUID, aiTurns)
		}
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

func (s *Service) grantToRecipient(
	ctx context.Context,
	supporterID pgtype.UUID,
	recipientID pgtype.UUID,
	turns int32,
) error {
	uid := formatUUID(recipientID)
	supporterUID := formatUUID(supporterID)
	_, err := s.usageSvc.GrantUsageQuota(ctx, billing.UsageGrantInput{
		UserID:      uid,
		CaredUserID: uid,
		UsageType:   billing.UsageAIMinutes,
		Quantity:    float64(turns),
		Unit:        "turn",
		Reason:      "supporter_redeem_direct",
		Source:      "supporter",
		SourceID:    fmt.Sprintf("supporter_redeem_%s", time.Now().Format("20060102150405")),
		GrantedByID: supporterUID,
	})
	if err != nil {
		slog.Warn("supporter_grant_direct_failed", "recipient", uid, "error", err)
	}
	return err
}

func (s *Service) grantToCaredUsers(
	ctx context.Context,
	supporterID pgtype.UUID,
	totalTurns int32,
) error {
	if totalTurns <= 0 {
		return nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT cgcu.user_id
		FROM care_group_cared_users cgcu
		JOIN care_group_members cgm ON cgm.care_group_id = cgcu.care_group_id
		WHERE cgm.user_id = $1 AND cgcu.deleted_at IS NULL AND cgm.deleted_at IS NULL
	`, supporterID)
	if err != nil {
		slog.Warn("supporter_list_cared_users_failed", "error", err)
		return err
	}
	defer rows.Close()

	var caredIDs []pgtype.UUID
	for rows.Next() {
		var uid pgtype.UUID
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		caredIDs = append(caredIDs, uid)
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	if len(caredIDs) == 0 {
		return nil
	}

	turnsPerUser := float64(totalTurns) / float64(len(caredIDs))
	supporterUID := formatUUID(supporterID)

	for _, cuid := range caredIDs {
		uid := formatUUID(cuid)
		_, err = s.usageSvc.GrantUsageQuota(ctx, billing.UsageGrantInput{
			UserID:      uid,
			CaredUserID: uid,
			UsageType:   billing.UsageAIMinutes,
			Quantity:    turnsPerUser,
			Unit:        "turn",
			Reason:      "supporter_redeem_community",
			Source:      "supporter",
			SourceID:    fmt.Sprintf("supporter_community_%s", time.Now().Format("20060102150405")),
			GrantedByID: supporterUID,
		})
		if err != nil {
			slog.Warn("supporter_grant_community_failed", "cared_user", uid, "error", err)
		}
	}
	return nil
}

func formatUUID(uid pgtype.UUID) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uid.Bytes[0:4], uid.Bytes[4:6], uid.Bytes[6:8], uid.Bytes[8:10], uid.Bytes[10:16])
}

type ReferralLinkResponse struct {
	ReferralLink string `json:"referral_link"`
	ReferralCode string `json:"referral_code"`
}

type ClaimReferralRequest struct {
	ReferrerID string `json:"referrer_id"`
}

type ClaimReferralResponse struct {
	CoinsGranted int32 `json:"coins_granted"`
}

const referralCoinsPerParty = 200

func (s *Service) GetReferralLink(ctx context.Context, userID string) (*ReferralLinkResponse, error) {
	return &ReferralLinkResponse{
		ReferralLink: fmt.Sprintf("https://aisee.app/invite?ref=%s", userID),
		ReferralCode: userID,
	}, nil
}

func (s *Service) ClaimReferral(ctx context.Context, refereeID pgtype.UUID, req ClaimReferralRequest) (*ClaimReferralResponse, error) {
	var referrerID pgtype.UUID
	if err := referrerID.Scan(req.ReferrerID); err != nil {
		return nil, fmt.Errorf("invalid referrer_id")
	}

	refereeUUID := formatUUID(refereeID)
	referrerUUID := formatUUID(referrerID)

	if refereeUUID == referrerUUID {
		return nil, ErrReferralSelfRefer
	}

	var existingCount int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM referrals WHERE referee_id = $1`, refereeID,
	).Scan(&existingCount); err != nil {
		return nil, fmt.Errorf("failed to check existing referral: %w", err)
	}
	if existingCount > 0 {
		return nil, ErrReferralAlreadyClaimed
	}

	var referrerExists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, referrerID,
	).Scan(&referrerExists); err != nil || !referrerExists {
		return nil, fmt.Errorf("referrer not found")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO referrals (referrer_id, referee_id) VALUES ($1, $2)`,
		referrerID, refereeID,
	); err != nil {
		return nil, fmt.Errorf("failed to insert referral: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO supporter_wallets (user_id, balance, total_earned, updated_at)
		 VALUES ($1, $2, $2, now())
		 ON CONFLICT (user_id) DO UPDATE SET
		     balance = supporter_wallets.balance + $2,
		     total_earned = supporter_wallets.total_earned + $2,
		     updated_at = now()`,
		referrerID, referralCoinsPerParty,
	); err != nil {
		return nil, fmt.Errorf("failed to credit referrer: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO supporter_wallets (user_id, balance, total_earned, updated_at)
		 VALUES ($1, $2, $2, now())
		 ON CONFLICT (user_id) DO UPDATE SET
		     balance = supporter_wallets.balance + $2,
		     total_earned = supporter_wallets.total_earned + $2,
		     updated_at = now()`,
		refereeID, referralCoinsPerParty,
	); err != nil {
		return nil, fmt.Errorf("failed to credit referee: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO supporter_transactions (user_id, type, amount)
		 VALUES ($1, 'referral_reward', $2)`,
		referrerID, referralCoinsPerParty,
	); err != nil {
		return nil, fmt.Errorf("failed to log referrer transaction: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO supporter_transactions (user_id, type, amount)
		 VALUES ($1, 'referral_reward', $2)`,
		refereeID, referralCoinsPerParty,
	); err != nil {
		return nil, fmt.Errorf("failed to log referee transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &ClaimReferralResponse{CoinsGranted: referralCoinsPerParty}, nil
}
