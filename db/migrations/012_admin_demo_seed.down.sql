-- 012_admin_demo_seed.down.sql

DELETE FROM ai_cost_events
WHERE source = 'admin_demo_seed'
   OR user_id IN (
      '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid,
      '2c2f8846-bd9a-4eb7-b895-7c82323db002'::uuid,
      '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid
   );

DELETE FROM usage_ledger
WHERE source = 'admin_demo_seed'
   OR user_id IN (
      '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid,
      '2c2f8846-bd9a-4eb7-b895-7c82323db002'::uuid,
      '90bb7db8-d730-4421-871e-97f8b20bb334'::uuid
   );

DELETE FROM subscriptions
WHERE metadata->>'seed' = 'admin_demo'
   OR id IN (
      'b3a3dc2b-8463-4803-88fa-f018087da950'::uuid,
      'fd6fef35-1a8a-4902-bc82-497683b25ad6'::uuid
   );

DELETE FROM family_links
WHERE id = '2af6e255-76c1-4f12-8970-9460ef214b8a'::uuid
   OR (
      user_id = '35c2b773-78c9-44cb-9b2d-7b5da977bd46'::uuid
      AND family_id = '2c2f8846-bd9a-4eb7-b895-7c82323db002'::uuid
   );

DELETE FROM users
WHERE google_id IN (
      'demo-google-nguyen-an',
      'demo-google-lan-pham',
      'demo-google-minh-tran'
   );
