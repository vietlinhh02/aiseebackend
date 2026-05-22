-- Cleanup orphan personal care groups created by the now-removed
-- AuthenticateGoogle.ensurePrimaryCareGroup bootstrap. A "user"-role account
-- ended up owning a private group containing only itself, blocking JoinFamily
-- with ErrGroupJoinRequiresLeaveCurrent even though the user had no real
-- family ties.

-- Detect groups owned by a role='user' account whose only live members /
-- cared users are the owner themselves (no family members ever joined).
WITH solo_groups AS (
    SELECT cg.id
    FROM care_groups cg
    JOIN users u ON u.id = cg.owner_user_id
    WHERE cg.status = 'active'
      AND u.role = 'user'
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_members cgm
          WHERE cgm.care_group_id = cg.id
            AND cgm.user_id <> cg.owner_user_id
            AND cgm.deleted_at IS NULL
      )
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_cared_users cgu
          WHERE cgu.care_group_id = cg.id
            AND cgu.user_id <> cg.owner_user_id
            AND cgu.deleted_at IS NULL
      )
)
UPDATE care_group_members
SET deleted_at = now()
WHERE deleted_at IS NULL
  AND care_group_id IN (SELECT id FROM solo_groups);

WITH solo_groups AS (
    SELECT cg.id
    FROM care_groups cg
    JOIN users u ON u.id = cg.owner_user_id
    WHERE cg.status = 'active'
      AND u.role = 'user'
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_members cgm
          WHERE cgm.care_group_id = cg.id
            AND cgm.user_id <> cg.owner_user_id
            AND cgm.deleted_at IS NULL
      )
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_cared_users cgu
          WHERE cgu.care_group_id = cg.id
            AND cgu.user_id <> cg.owner_user_id
            AND cgu.deleted_at IS NULL
      )
)
UPDATE care_group_cared_users
SET deleted_at = now()
WHERE deleted_at IS NULL
  AND care_group_id IN (SELECT id FROM solo_groups);

WITH solo_groups AS (
    SELECT cg.id
    FROM care_groups cg
    JOIN users u ON u.id = cg.owner_user_id
    WHERE cg.status = 'active'
      AND u.role = 'user'
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_members cgm
          WHERE cgm.care_group_id = cg.id
            AND cgm.user_id <> cg.owner_user_id
            AND cgm.deleted_at IS NULL
      )
      AND NOT EXISTS (
          SELECT 1
          FROM care_group_cared_users cgu
          WHERE cgu.care_group_id = cg.id
            AND cgu.user_id <> cg.owner_user_id
            AND cgu.deleted_at IS NULL
      )
)
UPDATE care_groups
SET status = 'archived', updated_at = now()
WHERE id IN (SELECT id FROM solo_groups);

-- Also clear residual orphan care_group_cared_users rows where the user has
-- no remaining family_links (matches the DeleteFamilyLink fix).
UPDATE care_group_cared_users cgu
SET deleted_at = now()
WHERE cgu.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM family_links fl
    WHERE fl.deleted_at IS NULL
      AND (fl.user_id = cgu.user_id OR fl.family_id = cgu.user_id)
  );
