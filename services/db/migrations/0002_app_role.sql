-- The services connect as a member of camera_app, which owns nothing and is
-- not a superuser, so row-level security always applies to it.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'camera_app') THEN
    CREATE ROLE camera_app NOLOGIN;
  END IF;
END $$;

GRANT USAGE ON SCHEMA public TO camera_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO camera_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO camera_app;
REVOKE UPDATE, DELETE ON audit_log FROM camera_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO camera_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO camera_app;

-- Devices waiting to be claimed. Platform table, no tenant yet.
CREATE TABLE device_inventory (
  serial           text PRIMARY KEY,
  hw_model         text NOT NULL,
  claim_code_hash  text NOT NULL,
  claimed_at       timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT, UPDATE ON device_inventory TO camera_app;
