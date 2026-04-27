-- db/migrations/008_revenuecat_events.up.sql

CREATE UNIQUE INDEX idx_subscription_events_provider_event
    ON subscription_events(provider, provider_event_id)
    WHERE provider IS NOT NULL AND provider_event_id IS NOT NULL;
