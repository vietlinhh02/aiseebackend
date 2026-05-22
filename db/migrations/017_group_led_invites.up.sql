ALTER TABLE family_invites
    ALTER COLUMN cared_user_id DROP NOT NULL;

ALTER TABLE family_invites
    ADD COLUMN care_group_id UUID REFERENCES care_groups(id) ON DELETE CASCADE,
    ADD COLUMN owner_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN used_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN used_at TIMESTAMPTZ;

UPDATE family_invites fi
SET
    care_group_id = cgu.care_group_id,
    owner_user_id = cg.owner_user_id
FROM care_group_cared_users cgu
JOIN care_groups cg ON cg.id = cgu.care_group_id
WHERE fi.cared_user_id = cgu.user_id
  AND cgu.deleted_at IS NULL
  AND fi.care_group_id IS NULL;

ALTER TABLE family_invites
    ALTER COLUMN care_group_id SET NOT NULL,
    ALTER COLUMN owner_user_id SET NOT NULL,
    ALTER COLUMN max_uses SET DEFAULT 1;

UPDATE family_invites
SET max_uses = 1
WHERE max_uses IS NULL;

ALTER TABLE family_invites
    ADD CONSTRAINT family_invites_code_six_digits_ck
    CHECK (code ~ '^[0-9]{6}$');

CREATE INDEX idx_family_invites_group
    ON family_invites(care_group_id, created_at DESC);

CREATE INDEX idx_family_invites_owner
    ON family_invites(owner_user_id, created_at DESC);
