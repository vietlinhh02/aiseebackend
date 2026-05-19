# Family-Led Group Invite And Subscription Rework

## Summary

AiSee currently hardcodes a patient-led family invite model:

- only `role = user` can create invites
- invites point to `cared_user_id`
- join flow assumes the joiner is always `family`
- `family_links` acts as both the source of truth and the compatibility surface
- billing is effectively resolved from one subscription path at a time

That model no longer matches the desired product:

- a `family` member creates the group first
- invite codes are 6 digits and single-use
- invitees may be `user` or `family`
- only the first inviter owns group management and billing
- subscription value from multiple family members contributes to one group
- quota totals are additive, while boolean/level entitlements take the strongest value
- users already in another group must leave before joining a new one
- the first inviter cannot leave before transferring ownership

This design moves the domain to a group-centric model while preserving the existing onboarding roles and role-specific UI.

## Product Rules

### Roles

- `user` remains the patient role chosen during onboarding.
- `family` remains the family/caregiver role chosen during onboarding.
- UI branching continues to use these existing roles.

### Group ownership

- The first `family` user who creates a group becomes the `owner`.
- Only the `owner` can:
  - create invite codes
  - revoke invite codes
  - manage billing and subscriptions
  - transfer ownership
  - invite additional members or patients
- A non-owner can view group status and leave the group, but cannot manage invites or billing.

### Invite behavior

- Invite code format becomes exactly 6 digits.
- Each code is single-use.
- Each code can be entered by either `user` or `family`.
- The role of the person entering the code determines whether they join as a cared user or a family member.

### Group membership

- A `user` joining by invite is added to `care_group_cared_users`.
- A `family` joining by invite is added to `care_group_members`.
- A person already in another active group must leave that group before joining a new one.

### Leaving a group

- Both `user` and `family` members can leave a group.
- The `owner` cannot leave while they still own the group.
- To leave, the `owner` must first transfer ownership to another `family` member.

### Billing and subscriptions

- Billing is group-based, not person-based.
- Only the `owner` can manage billing UI and purchase actions.
- Multiple family members may each have their own subscription records.
- When those members belong to the same group:
  - numeric quotas are summed across active subscriptions
  - boolean entitlements use logical OR / strongest enabled state
  - level-based entitlements use the highest supported level
- If a contributing family member leaves the group, their subscription no longer contributes to the old group aggregate.

## Root Cause Of The Current Error

The current join flow assumes:

- invite creator is always a patient
- invite target is always a family member
- `family_links` insertion happens directly as:
  - `user_id = cared_user_id`
  - `family_id = joiner_id`

After resetting data and testing a reversed flow, that assumption is no longer valid. The system reaches `upsertFamilyLinkTx` with ids that do not match the expected domain direction, which surfaces as:

- `insert or update on table "family_links" violates foreign key constraint "family_links_family_id_fkey"`

The fix is not a narrow FK patch. The join flow must resolve group membership first, then write any compatibility projection such as `family_links`.

## Domain Model Changes

### Source of truth

`care_groups`, `care_group_members`, and `care_group_cared_users` become the source of truth for membership and ownership.

`family_links` becomes a compatibility projection used by existing chat/call/care paths until those paths are fully migrated to group-based lookup.

### Invite model

Replace the patient-led invite semantics with a group-led invite record.

Recommended shape:

- `code`
- `care_group_id`
- `owner_user_id`
- `created_by_user_id`
- `relationship_label`
- `target_role = 'any'`
- `expires_at`
- `revoked_at`
- `used_by_user_id`
- `used_at`
- `max_uses = 1`
- `usage_count`

This can be implemented by migrating the existing `family_invites` table or replacing it with a renamed group invite table while preserving API surface where practical.

### Ownership

Add explicit ownership semantics to care groups:

- existing `care_groups.owner_user_id` remains the canonical owner field
- owner must always be a `family` member in `care_group_members`
- ownership transfer updates both the owner field and member role state consistently

## Backend Flow Design

### Create invite

`POST /api/v1/users/family-link`

New behavior:

- requester must be `family`
- requester must be current group owner, or if no group exists yet:
  - create a new group
  - add requester as `owner`
- generate a 6-digit single-use code
- store invite against the group, not against a cared user

### Join by code

`POST /api/v1/users/join-family`

New flow:

1. Normalize and validate the 6-digit code.
2. Lock invite row for update.
3. Reject if revoked, expired, or already used.
4. Check whether the joiner is already in another active group.
5. If yes, return `group_join_requires_leave_current`.
6. Resolve joiner role from the authenticated user record.
7. If joiner role is `user`:
   - add to `care_group_cared_users`
8. If joiner role is `family`:
   - add to `care_group_members` with non-owner role
9. Mark invite used.
10. Update compatibility projections such as `family_links`.

### Leave group

Add a dedicated leave-group endpoint.

Recommended API:

- `POST /api/v1/users/leave-group`

Behavior:

- for `user`: remove or soft-delete cared-user membership from the active group
- for non-owner `family`: remove or soft-delete group membership
- for owner: reject with `group_owner_transfer_required`

### Transfer ownership

Add explicit owner transfer endpoint.

Recommended API:

- `POST /api/v1/users/transfer-group-ownership`

Behavior:

- requester must be current owner
- target user must be an active `family` member of the same group
- update `care_groups.owner_user_id`
- demote old owner to non-owner family role
- promote target to owner/billing-admin role

### Compatibility projection for `family_links`

Existing app/backend logic still uses `family_links` for:

- family contact resolution
- chat/call contact lists
- care authorization checks

Short-term design:

- after a successful join, regenerate or upsert `family_links` from group membership
- keep link direction predictable:
  - `user_id` always points to the patient
  - `family_id` always points to the family member

For a group with multiple patients and multiple family members, create links for each valid patient-family pair required by legacy flows.

## Billing Aggregation Design

### Ownership and management

- Billing actions remain restricted to the care group owner.
- Subscription purchase records remain attached to the actual purchaser user.

### Group aggregate read model

Billing overview and quota checks should resolve through a new group aggregate layer:

1. Find the active care group.
2. Find active subscriptions owned by family members in that group.
3. Map each subscription to its plan entitlements.
4. Aggregate entitlements into one effective group quota model.

### Aggregation rules

#### Numeric quotas

Sum all included numeric quantities across active group subscriptions.

Examples:

- `ai_minutes_per_month`
- `camera_activations_per_day`
- `voice_call_minutes_per_month`
- `video_call_minutes_per_month`
- `pdf_exports_per_month`
- `care_coach_questions_per_month`

#### Boolean entitlements

Use the strongest enabled value.

Examples:

- `weekly_report_enabled`
- `daily_brief_enabled`
- `smart_alert_enabled`
- `priority_support_enabled`

#### Level/string entitlements

Use the highest value according to a fixed ordering table.

Examples:

- `permission_matrix_level`: `none < basic < advanced`
- `volunteer_priority_level`: `none < standard < priority < highest`
- `export_format`: `none < csv < pdf_csv`

### Quota checks

All quota checks should resolve against the group aggregate, not a single selected subscription.

### When a member leaves

- their subscription records stay attached to that user
- the old group no longer counts those subscriptions in the aggregate
- the new group will count them only after the user joins and becomes an active family member there

## UI Design

### Family app experience

#### Family owner

Show:

- create 6-digit invite code
- revoke invite code
- group member list
- cared user list
- subscription summary
- aggregate quota summary
- ownership transfer
- leave group action

Hide from non-owner:

- billing management
- invite management
- ownership transfer

#### Non-owner family member

Show:

- current group status
- current patients and family members
- aggregate subscription/quota summary
- leave group action

### User app experience

- Main landing remains the AI conversation experience.
- Join-group entry moves to the existing messaging tab next to the AI screen.
- After joining a group, user UI remains patient-oriented but shows:
  - current group status
  - linked family members
  - relevant subscription/quota context where already exposed in the UI

### Shared UI expectations

- Both roles can see current group status.
- Only the owner sees billing and invite management actions.
- Join errors must be user-readable and role-aware.

## API And Error Contract

### Existing endpoints to keep and repurpose

- `POST /api/v1/users/family-link`
- `POST /api/v1/users/join-family`
- `DELETE /api/v1/users/family-invite/{invite_code}`

### New endpoints

- `POST /api/v1/users/leave-group`
- `POST /api/v1/users/transfer-group-ownership`
- optionally `GET /api/v1/users/me/group` for a direct group summary payload

### Error codes

- `invite_invalid`
- `invite_expired`
- `invite_already_used`
- `invite_revoke_forbidden`
- `group_join_requires_leave_current`
- `group_owner_only`
- `group_owner_transfer_required`
- `group_member_already_joined`

## Migration Strategy

### Database

1. Add or reshape invite table columns for group-led invites.
2. Add indexes for 6-digit code lookup and single-use enforcement.
3. Add any missing fields needed for ownership transfer and leave-group auditing.
4. Keep current plans and subscriptions schema intact.

### Data backfill

For existing environments:

- preserve current care groups
- infer current owners from existing group owner field
- preserve current patient/family memberships
- retire old patient-led invite semantics

### Compatibility rollout

Do not remove `family_links` immediately.

First phase:

- make group membership the source of truth
- maintain `family_links` as derived compatibility data

Second phase:

- migrate downstream features to query group membership directly
- reduce `family_links` to a legacy support artifact or remove it later

## Implementation Order

1. Migrate invite/domain model and add leave-group + ownership-transfer APIs.
2. Rewrite join flow to be group-first and role-aware.
3. Add compatibility projection updates for `family_links`.
4. Implement care-group subscription aggregation.
5. Update family UI for owner vs non-owner management.
6. Update user UI to place join-group entry in the messaging tab.
7. Update analytics, error copy, and regression tests.

## Testing Strategy

### Backend unit tests

- 6-digit invite generation
- single-use invite consumption
- owner-only invite creation
- `user` joins as cared user
- `family` joins as family member
- reject join when user is already in another group
- owner must transfer ownership before leaving
- aggregate numeric quotas by sum
- aggregate boolean/level entitlements by strongest value

### Backend integration tests

- join flow updates membership tables correctly
- compatibility `family_links` projection is created with correct direction
- chat/call/care lookups still resolve after new join flow
- leaving a group removes subscription contribution from the aggregate

### App tests

- family owner sees invite/billing controls
- non-owner family member does not
- user sees join entry in messaging tab
- error messaging for join/leave/ownership transfer is correct

### Regression coverage

- chat
- calls
- family dashboard
- billing overview
- quota checks

## Risks

- `family_links` compatibility logic can become expensive or subtle for groups with many users and family members.
- Aggregating entitlements across multiple subscriptions introduces new plan-combination edge cases.
- Legacy code paths that still assume a single patient-family pair may need follow-up cleanup after the first migration.

## Recommended Scope Boundary

This change should be treated as one coordinated rework across:

- `aiseebackend`
- `aiseeapp`
- any family web/admin views that surface group or subscription state

Do not implement it as a narrow FK fix. The FK issue is a symptom of the old invite model, not the root problem.
