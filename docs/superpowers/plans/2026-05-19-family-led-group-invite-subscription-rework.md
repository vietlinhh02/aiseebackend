# Family-Led Group Invite And Subscription Rework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the patient-led invite flow with a family-led care-group model, add leave-group and ownership transfer, aggregate group subscriptions, and update app UI to match the new ownership and join rules.

**Architecture:** The backend becomes group-centric: `care_groups` and membership tables are the source of truth, while `family_links` is maintained as a compatibility projection for legacy call/chat/care flows. Billing reads aggregate all active family-member subscriptions in a group into one effective quota model, and the Flutter app surfaces owner-only management actions while keeping the patient landing flow unchanged.

**Tech Stack:** Go, PostgreSQL migrations, sqlc-generated DB access plus raw pgx SQL, Flutter/Dart, Go/Flutter tests

---

### Task 1: Migrate invite and group ownership schema

**Files:**
- Modify: `aiseebackend/db/migrations/016_family_invites.up.sql`
- Modify: `aiseebackend/db/migrations/016_family_invites.down.sql`
- Create: `aiseebackend/db/migrations/017_group_membership_controls.up.sql`
- Create: `aiseebackend/db/migrations/017_group_membership_controls.down.sql`
- Test: `aiseebackend/internal/user/service_test.go`

- [ ] Add group-led invite columns (`care_group_id`, `owner_user_id`, `used_by_user_id`, `used_at`) and constrain invite codes to 6 digits.
- [ ] Add indexes for active invite lookup and active membership lookup by user across care groups.
- [ ] Add or update tests that exercise code normalization and 6-digit generation expectations.

### Task 2: Rewrite backend invite creation and join flow

**Files:**
- Modify: `aiseebackend/internal/user/invite.go`
- Modify: `aiseebackend/internal/user/service.go`
- Modify: `aiseebackend/internal/user/handler.go`
- Test: `aiseebackend/internal/user/service_test.go`

- [ ] Write failing tests for: family owner can create invite, non-family/non-owner cannot, invite code is 6 digits, `user` join enters cared-users table, `family` join enters members table, invite becomes single-use.
- [ ] Run targeted Go tests and verify they fail for the expected old behavior.
- [ ] Implement minimal group-led invite creation and role-aware join logic.
- [ ] Keep `family_links` as a derived compatibility projection instead of the primary membership write path.
- [ ] Re-run targeted Go tests until green.

### Task 3: Add leave-group and ownership transfer backend APIs

**Files:**
- Modify: `aiseebackend/internal/user/service.go`
- Modify: `aiseebackend/internal/user/handler.go`
- Modify: `aiseebackend/cmd/server/main.go`
- Test: `aiseebackend/internal/user/service_test.go`

- [ ] Write failing tests for leaving a group, blocking join when member is already in another group, blocking owner leave until transfer, and successful ownership transfer to another family member.
- [ ] Implement service methods and handlers for `POST /api/v1/users/leave-group` and `POST /api/v1/users/transfer-group-ownership`.
- [ ] Verify tests pass and error codes match the spec.

### Task 4: Aggregate subscriptions by care group

**Files:**
- Modify: `aiseebackend/internal/billing/store.go`
- Modify: `aiseebackend/internal/billing/service.go`
- Modify: `aiseebackend/internal/billing/types.go`
- Test: `aiseebackend/internal/billing/service_test.go`

- [ ] Write failing tests for summed numeric quotas and strongest-value entitlement aggregation across multiple family subscriptions in one care group.
- [ ] Implement subscription lookup across active family members in the group.
- [ ] Implement aggregation helpers for numeric, boolean, and ordered level entitlements.
- [ ] Route quota checks and billing overview reads through the aggregate.
- [ ] Re-run billing tests and verify green.

### Task 5: Update Flutter family and patient UI

**Files:**
- Modify: `aiseeapp/lib/services/aisee_family_service.dart`
- Modify: `aiseeapp/lib/screens/contacts_tab.dart`
- Modify: `aiseeapp/lib/screens/family_contacts_tab.dart`
- Modify: `aiseeapp/lib/models/aisee_family_member.dart`
- Modify: `aiseeapp/lib/models/aisee_api_models.dart`
- Test: `aiseeapp/test/family_contacts_tab_test.dart`
- Test: `aiseeapp/test/user_sponsor_screen_test.dart`

- [ ] Write failing Flutter tests for owner-only invite controls, patient join entry in the messaging tab, and updated error strings.
- [ ] Update service calls and UI copy for 6-digit join codes and owner-only group management.
- [ ] Add leave-group and ownership-transfer actions to family UI where appropriate.
- [ ] Re-run targeted Flutter tests until green.

### Task 6: Verify compatibility and regressions

**Files:**
- Modify as needed: `aiseebackend/internal/call/service_test.go`
- Modify as needed: `aiseebackend/internal/care/integration_db_test.go`
- Test: existing Go and Flutter suites

- [ ] Add or update regression coverage for call/chat/care flows that still depend on `family_links`.
- [ ] Run `go test ./internal/user ./internal/billing ./internal/call ./internal/care`.
- [ ] Run `go test ./...`.
- [ ] Run targeted Flutter tests for the updated invite/group screens.
- [ ] Perform a completion audit against the spec before claiming success.
