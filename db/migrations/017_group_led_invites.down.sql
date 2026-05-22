DROP INDEX IF EXISTS idx_family_invites_owner;
DROP INDEX IF EXISTS idx_family_invites_group;

ALTER TABLE family_invites
    DROP CONSTRAINT IF EXISTS family_invites_code_six_digits_ck;

ALTER TABLE family_invites
    ALTER COLUMN max_uses DROP DEFAULT,
    DROP COLUMN IF EXISTS used_at,
    DROP COLUMN IF EXISTS used_by_user_id,
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS care_group_id;

DELETE FROM family_invites
WHERE cared_user_id IS NULL;

ALTER TABLE family_invites
    ALTER COLUMN cared_user_id SET NOT NULL;
