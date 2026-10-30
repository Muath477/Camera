-- Acknowledgement notes, phone sign-in and notification delivery.

ALTER TABLE events ADD COLUMN ack_note text;

-- One-time sign-in codes. Platform table: a phone is not tied to a tenant
-- until it is verified. Codes are stored hashed and expire in minutes.
CREATE TABLE otp_codes (
  id          bigserial PRIMARY KEY,
  phone       text NOT NULL,
  code_hash   text NOT NULL,
  attempts    int NOT NULL DEFAULT 0,
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_phone ON otp_codes (phone, created_at DESC);

-- Which users a phone number signs in as, across tenants. Kept in step with
-- users by trigger, so sign-in can find the tenant before it has one.
CREATE TABLE phone_logins (
  phone      text NOT NULL,
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (phone, tenant_id)
);

CREATE OR REPLACE FUNCTION sync_phone_login() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND OLD.phone IS DISTINCT FROM NEW.phone THEN
    DELETE FROM phone_logins WHERE user_id = OLD.id;
  END IF;
  IF NEW.phone IS NOT NULL THEN
    INSERT INTO phone_logins (phone, tenant_id, user_id) VALUES (NEW.phone, NEW.tenant_id, NEW.id)
    ON CONFLICT (phone, tenant_id) DO UPDATE SET user_id = excluded.user_id;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER users_phone_login AFTER INSERT OR UPDATE OF phone ON users
  FOR EACH ROW EXECUTE FUNCTION sync_phone_login();
INSERT INTO phone_logins (phone, tenant_id, user_id)
  SELECT phone, tenant_id, id FROM users WHERE phone IS NOT NULL
  ON CONFLICT DO NOTHING;

-- Browser and phone push subscriptions.
CREATE TABLE push_subscriptions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint    text NOT NULL UNIQUE,
  p256dh      text NOT NULL,
  auth        text NOT NULL,
  user_agent  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);

-- How each user wants to be reached. Critical alerts ignore quiet hours;
-- warnings wait until they end; informational events stay in the app.
CREATE TABLE notification_prefs (
  tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id           uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  critical_channels text[] NOT NULL DEFAULT '{push,sms}',
  warning_channels  text[] NOT NULL DEFAULT '{push}',
  quiet_from        time,
  quiet_to          time,
  updated_at        timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['push_subscriptions','notification_prefs']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
  END LOOP;
END $$;

GRANT SELECT, INSERT, UPDATE, DELETE ON otp_codes, phone_logins, push_subscriptions, notification_prefs TO camera_app;
GRANT USAGE, SELECT ON SEQUENCE otp_codes_id_seq TO camera_app;
