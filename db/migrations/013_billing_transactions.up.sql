-- 013_billing_transactions.up.sql

CREATE TABLE billing_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    care_group_id UUID REFERENCES care_groups(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID REFERENCES plans(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    status TEXT NOT NULL,
    amount NUMERIC(12,2) NOT NULL,
    currency TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_billing_transactions_provider_event
    ON billing_transactions(provider, provider_event_id);

CREATE INDEX idx_billing_transactions_user_occurred_at
    ON billing_transactions(user_id, occurred_at DESC);

CREATE INDEX idx_billing_transactions_care_group_occurred_at
    ON billing_transactions(care_group_id, occurred_at DESC);
