CREATE TABLE IF NOT EXISTS supporter_wallets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance INTEGER NOT NULL DEFAULT 0 CHECK (balance >= 0),
    total_earned INTEGER NOT NULL DEFAULT 0,
    total_redeemed INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supporter_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('watch_ad', 'redeem', 'grant')),
    amount INTEGER NOT NULL,
    recipient_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_supporter_txs_user_id ON supporter_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_supporter_txs_type ON supporter_transactions(type);

CREATE TABLE IF NOT EXISTS supporter_daily_ad_watches (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    watch_date DATE NOT NULL DEFAULT CURRENT_DATE,
    count INTEGER NOT NULL DEFAULT 0 CHECK (count >= 0),
    PRIMARY KEY (user_id, watch_date)
);
