-- db/migrations/004_billing_plans.down.sql

DROP TABLE IF EXISTS subscription_events;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plan_entitlements;
DROP TABLE IF EXISTS plans;
