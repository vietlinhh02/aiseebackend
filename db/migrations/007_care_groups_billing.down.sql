-- db/migrations/007_care_groups_billing.down.sql

DROP INDEX IF EXISTS idx_subscription_events_care_group;
DROP INDEX IF EXISTS idx_subscriptions_care_group_status;
DROP INDEX IF EXISTS idx_care_group_cared_users_group;
DROP INDEX IF EXISTS idx_care_group_cared_users_user;
DROP INDEX IF EXISTS idx_care_group_members_group;
DROP INDEX IF EXISTS idx_care_group_members_user;
DROP INDEX IF EXISTS idx_care_groups_owner;

ALTER TABLE subscription_events DROP COLUMN IF EXISTS care_group_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS care_group_id;

DROP TABLE IF EXISTS care_group_cared_users;
DROP TABLE IF EXISTS care_group_members;
DROP TABLE IF EXISTS care_groups;
