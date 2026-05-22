INSERT INTO users (id, google_id, email, display_name, role)
VALUES ('00000000-0000-0000-0000-000000000000', 'system-admin', 'admin@aaisee.me', 'System Admin', 'admin')
ON CONFLICT (id) DO NOTHING;

INSERT INTO admin_users (user_id, email)
VALUES ('00000000-0000-0000-0000-000000000000', 'admin@aaisee.me')
ON CONFLICT (user_id) DO NOTHING;
