package billing

import (
	"os"
	"strings"
	"testing"
)

func TestBillingMigrationMatchesCarePlanConfig(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/004_billing_plans.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	body := string(sql)

	required := []string{
		"('free', 'Free / Essential', 0, 'month', 'active', 0)",
		"('basic_149k', 'Care Basic', 149000, 'month', 'active', 10)",
		"('plus_399k', 'Care Plus', 399000, 'month', 'active', 20)",
		"('premium_599k', 'Care Premium', 599000, 'month', 'active', 30)",
		"('basic_149k', 'ai_minutes_per_month', '150'::jsonb, 'minutes', 'month'",
		"('plus_399k', 'ai_minutes_per_month', '600'::jsonb, 'minutes', 'month'",
		"('basic_149k', 'camera_activations_per_day', '3'::jsonb, 'activations', 'day'",
		"('plus_399k', 'camera_activations_per_day', '20'::jsonb, 'activations', 'day'",
		"('basic_149k', 'family_members_limit', '2'::jsonb, 'members', 'never'",
		"('plus_399k', 'family_members_limit', '5'::jsonb, 'members', 'never'",
		"('premium_599k', 'family_members_limit', '10'::jsonb, 'members', 'never'",
		"('basic_149k', 'care_timeline_days', '7'::jsonb, 'days', 'never'",
		"('plus_399k', 'care_timeline_days', '90'::jsonb, 'days', 'never'",
		"('premium_599k', 'care_timeline_days', '365'::jsonb, 'days', 'never'",
		"('plus_399k', 'permission_matrix_level', '\"basic\"'::jsonb, NULL, 'never'",
		"('premium_599k', 'permission_matrix_level', '\"advanced\"'::jsonb, NULL, 'never'",
		"('premium_599k', 'export_format', '\"pdf_csv\"'::jsonb, NULL, 'never'",
		"('premium_599k', 'priority_support_enabled', 'true'::jsonb, NULL, 'never'",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Fatalf("migration missing care-plan config: %s", want)
		}
	}

	forbidden := []string{
		"('basic_149k', 'ai_minutes_per_month', '300'::jsonb",
		"('basic_149k', 'family_members_limit', '5'::jsonb",
		"('basic_149k', 'care_timeline_days', '180'::jsonb",
		"('basic_149k', 'permission_matrix_enabled', 'true'::jsonb",
	}
	for _, stale := range forbidden {
		if strings.Contains(body, stale) {
			t.Fatalf("migration still contains stale config: %s", stale)
		}
	}
}

func TestCareGroupBillingMigrationBackfillsLegacyBillingRows(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/007_care_groups_billing.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	body := string(sql)

	required := []string{
		"UPDATE subscriptions s",
		"SET care_group_id = ranked.care_group_id",
		"AND s.care_group_id IS NULL",
		"UPDATE subscription_events se",
		"SET care_group_id = s.care_group_id",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Fatalf("care-group migration missing legacy backfill: %s", want)
		}
	}
}

func TestBillingTransactionsMigrationCreatesProviderEventLedger(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/013_billing_transactions.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	body := string(sql)

	required := []string{
		"CREATE TABLE billing_transactions",
		"provider_event_id TEXT NOT NULL",
		"status TEXT NOT NULL",
		"amount NUMERIC(12,2) NOT NULL",
		"currency TEXT NOT NULL",
		"payload JSONB NOT NULL DEFAULT '{}'::jsonb",
		"CREATE UNIQUE INDEX idx_billing_transactions_provider_event",
		"ON billing_transactions(provider, provider_event_id)",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Fatalf("billing transaction migration missing: %s", want)
		}
	}
}

func TestAdminDemoSeedMigrationRequiresExplicitOptIn(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/012_admin_demo_seed.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	body := string(sql)

	required := []string{
		"current_setting('app.seed_demo_data', true)",
		"seed_guard AS (",
		"WHERE seed_guard.enabled",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Fatalf("admin demo seed migration missing guard: %s", want)
		}
	}
}
