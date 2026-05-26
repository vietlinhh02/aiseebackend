-- name: GetSupporterWallet :one
SELECT * FROM supporter_wallets WHERE user_id = $1;

-- name: UpsertSupporterWallet :exec
INSERT INTO supporter_wallets (user_id, balance, total_earned, updated_at)
VALUES ($1, $2, $2, now())
ON CONFLICT (user_id) DO UPDATE SET
    balance = supporter_wallets.balance + $2,
    total_earned = supporter_wallets.total_earned + $2,
    updated_at = now();

-- name: DeductSupporterBalance :exec
UPDATE supporter_wallets
SET balance = balance - $2, total_redeemed = total_redeemed + $2, updated_at = now()
WHERE user_id = $1 AND balance >= $2;

-- name: InsertSupporterTransaction :exec
INSERT INTO supporter_transactions (user_id, type, amount, recipient_user_id)
VALUES ($1, $2, $3, $4);

-- name: EnsureAndGetDailyAdWatch :one
INSERT INTO supporter_daily_ad_watches (user_id, watch_date, count)
VALUES ($1, CURRENT_DATE, 0)
ON CONFLICT (user_id, watch_date) DO UPDATE SET count = supporter_daily_ad_watches.count
RETURNING *;

-- name: IncrementDailyAdWatch :exec
UPDATE supporter_daily_ad_watches SET count = count + 1
WHERE user_id = $1 AND watch_date = CURRENT_DATE;

-- name: CountSupportedPeople :one
SELECT COUNT(DISTINCT cgcu.user_id)::INTEGER AS count
FROM care_group_cared_users cgcu
JOIN care_group_members cgm ON cgm.care_group_id = cgcu.care_group_id
WHERE cgm.user_id = $1 AND cgcu.deleted_at IS NULL AND cgm.deleted_at IS NULL;

-- name: GetPersonalSupporterStats :one
SELECT
    COALESCE(w.balance, 0)::INTEGER AS coin_balance,
    COALESCE(w.total_earned, 0)::INTEGER AS total_earned,
    COALESCE(w.total_redeemed, 0)::INTEGER AS total_redeemed,
    COALESCE(adw.count, 0)::INTEGER AS ads_watched_today
FROM (SELECT $1::UUID AS uid) AS u
LEFT JOIN supporter_wallets w ON w.user_id = u.uid
LEFT JOIN supporter_daily_ad_watches adw ON adw.user_id = u.uid AND adw.watch_date = CURRENT_DATE;

-- name: GetCommunitySupporterStats :one
SELECT
    (SELECT COUNT(DISTINCT user_id) FROM care_group_cared_users WHERE deleted_at IS NULL)::INTEGER AS total_cared_users,
    COALESCE((SELECT SUM(amount) FROM supporter_transactions WHERE type = 'redeem'), 0)::INTEGER AS total_ai_turns_gifted,
    COALESCE((SELECT COUNT(DISTINCT user_id) FROM supporter_daily_ad_watches WHERE watch_date > CURRENT_DATE - INTERVAL '30 days'), 0)::INTEGER AS active_supporters;

-- name: ListRecentCommunitySupporters :many
SELECT
    u.id,
    COALESCE(NULLIF(u.display_name, ''), u.email) AS display_name,
    u.avatar_url
FROM supporter_daily_ad_watches adw
JOIN users u ON u.id = adw.user_id
WHERE adw.watch_date > CURRENT_DATE - INTERVAL '30 days'
GROUP BY u.id, u.display_name, u.email, u.avatar_url
ORDER BY MAX(adw.watch_date) DESC, COALESCE(NULLIF(u.display_name, ''), u.email)
LIMIT $1;

-- name: GetTotalAITurnsGiftedByUser :one
SELECT COALESCE(SUM(amount), 0)::INTEGER AS total
FROM supporter_transactions
WHERE user_id = $1 AND type = 'redeem';
