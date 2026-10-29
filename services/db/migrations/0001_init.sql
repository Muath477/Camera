-- Core schema. Every tenant-owned table carries tenant_id and is protected by
-- row-level security keyed on the app.tenant_id setting, which the services
-- set per transaction from the caller's token. The service role is not the
-- table owner, and FORCE makes the policies apply to the owner as well.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION app_tenant() RETURNS uuid
LANGUAGE sql STABLE AS $$
  SELECT nullif(current_setting('app.tenant_id', true), '')::uuid
$$;

-- Tenancy -------------------------------------------------------------------

CREATE TABLE tenants (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL,
  cr_number   text,
  plan        text NOT NULL DEFAULT 'trial',
  status      text NOT NULL DEFAULT 'active',
  country     text NOT NULL DEFAULT 'SA',
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sites (
  id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id                uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name                     text NOT NULL,
  facility_category        text NOT NULL,
  address                  text,
  geom                     geography(Point, 4326),
  timezone                 text NOT NULL DEFAULT 'Asia/Riyadh',
  retention_days_required  int  NOT NULL DEFAULT 90,
  created_at               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sites_tenant ON sites (tenant_id);
CREATE INDEX sites_geom ON sites USING gist (geom);

CREATE TABLE floor_plans (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  site_id     uuid NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  name        text NOT NULL,
  image_key   text NOT NULL,
  width_m     real,
  height_m    real,
  calibration jsonb
);

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  keycloak_sub  text NOT NULL UNIQUE,
  name          text NOT NULL,
  phone         text,
  email         text,
  locale        text NOT NULL DEFAULT 'ar',
  mfa_enrolled  boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now()
);

-- site_id NULL means the role applies to every site of the tenant.
CREATE TABLE memberships (
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  site_id     uuid REFERENCES sites(id) ON DELETE CASCADE,
  role        text NOT NULL CHECK (role IN ('owner','site_manager','operator','viewer','auditor','integrator')),
  granted_by  uuid REFERENCES users(id),
  expires_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX memberships_unique ON memberships (user_id, coalesce(site_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- Devices and cameras --------------------------------------------------------

CREATE TABLE devices (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  site_id           uuid REFERENCES sites(id) ON DELETE SET NULL,
  serial            text NOT NULL UNIQUE,
  hw_model          text,
  cert_fingerprint  text,
  claim_code_hash   text,
  fw_version        text,
  channel           text NOT NULL DEFAULT 'stable',
  status            text NOT NULL DEFAULT 'pending',
  last_seen_at      timestamptz,
  disk_total        bigint,
  disk_used         bigint,
  retention_days    real,
  health            jsonb,
  created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX devices_tenant ON devices (tenant_id);

-- Platform table: no tenant, readable by everyone, pushed to devices.
CREATE TABLE camera_profiles (
  id                    text PRIMARY KEY,
  vendor                text NOT NULL,
  model_pattern         text,
  api_kind              text NOT NULL,
  rtsp_templates        jsonb NOT NULL,
  quirks                jsonb NOT NULL DEFAULT '[]',
  recommended_settings  jsonb NOT NULL DEFAULT '{}',
  verified              boolean NOT NULL DEFAULT false
);

CREATE TABLE cameras (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  device_id      uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  site_id        uuid NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  name           text NOT NULL,          -- the Frigate camera id on the device
  display_name   text NOT NULL,
  vendor         text,
  model          text,
  profile_id     text REFERENCES camera_profiles(id),
  onvif_capable  boolean NOT NULL DEFAULT false,
  ptz            boolean NOT NULL DEFAULT false,
  geom           geography(Point, 4326),
  floor_plan_id  uuid REFERENCES floor_plans(id) ON DELETE SET NULL,
  plan_x         real,
  plan_y         real,
  heading_deg    real,
  fov_deg        real,
  status         text NOT NULL DEFAULT 'unknown',
  article2_zone  text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (device_id, name)
);
CREATE INDEX cameras_site ON cameras (tenant_id, site_id);

CREATE TABLE zones (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  camera_id      uuid NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
  name           text NOT NULL,
  polygon        jsonb NOT NULL,
  article2_type  text,
  rules          jsonb NOT NULL DEFAULT '{}'
);

-- Events (TimescaleDB hypertable) ------------------------------------------

CREATE TABLE events (
  time          timestamptz NOT NULL,
  tenant_id     uuid NOT NULL,
  site_id       uuid NOT NULL,
  device_id     uuid NOT NULL,
  camera_id     uuid,
  edge_id       text NOT NULL,          -- id assigned on the device, unique per device
  kind          text NOT NULL,          -- object | audio | system
  label         text NOT NULL,
  score         real,
  zones         text[],
  end_time      timestamptz,
  snapshot_key  text,
  clip_key      text,
  embedding     vector(512),
  severity      text NOT NULL DEFAULT 'info',
  acked_by      uuid,
  acked_at      timestamptz,
  PRIMARY KEY (tenant_id, device_id, edge_id, time)
);
-- TimescaleDB is required in staging and production. It is optional here so
-- the schema also loads on a plain PostgreSQL with pgvector and PostGIS.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb') THEN
    CREATE EXTENSION IF NOT EXISTS timescaledb;
    PERFORM create_hypertable('events', 'time', chunk_time_interval => interval '1 day');
  END IF;
END $$;
CREATE INDEX events_tenant_time ON events (tenant_id, time DESC);
CREATE INDEX events_camera_time ON events (camera_id, time DESC);
CREATE INDEX events_label ON events (tenant_id, label, time DESC);
CREATE INDEX events_embedding ON events USING hnsw (embedding vector_cosine_ops);

CREATE TABLE snapshots (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  camera_id     uuid NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
  taken_at      timestamptz NOT NULL,
  requested_by  uuid REFERENCES users(id),
  s3_key        text NOT NULL,
  sha256        text,
  signature     text,
  source        text NOT NULL CHECK (source IN ('live','recording','event')),
  status        text NOT NULL DEFAULT 'pending',
  exported_at   timestamptz
);

CREATE TABLE alert_rules (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  site_id     uuid REFERENCES sites(id) ON DELETE CASCADE,
  name        text NOT NULL,
  conditions  jsonb NOT NULL,
  severity    text NOT NULL DEFAULT 'warning',
  channels    text[] NOT NULL DEFAULT '{push}',
  cooldown_s  int NOT NULL DEFAULT 300,
  enabled     boolean NOT NULL DEFAULT true
);

CREATE TABLE compliance_checks (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  site_id     uuid NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  check_type  text NOT NULL,
  status      text NOT NULL,
  evidence    jsonb NOT NULL DEFAULT '{}',
  checked_at  timestamptz NOT NULL DEFAULT now(),
  next_due    timestamptz
);
CREATE INDEX compliance_site ON compliance_checks (site_id, check_type, checked_at DESC);

-- Append-only, hash-chained audit trail.
CREATE TABLE audit_log (
  id           bigserial PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  actor_type   text NOT NULL,
  actor_id     text NOT NULL,
  action       text NOT NULL,
  target_type  text,
  target_id    text,
  ip           inet,
  device_info  text,
  details      jsonb,
  at           timestamptz NOT NULL DEFAULT now(),
  prev_hash    text NOT NULL,
  hash         text NOT NULL
);
CREATE INDEX audit_tenant ON audit_log (tenant_id, id);

CREATE TABLE api_keys (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name         text NOT NULL,
  scopes       text[] NOT NULL,
  secret_hash  text NOT NULL,
  last_used    timestamptz,
  status       text NOT NULL DEFAULT 'active'
);

CREATE TABLE webhooks (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  url         text NOT NULL,
  secret      text NOT NULL,
  events      text[] NOT NULL DEFAULT '{event.created}',
  last_used   timestamptz,
  status      text NOT NULL DEFAULT 'active'
);

-- Row-level security --------------------------------------------------------

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['sites','floor_plans','users','memberships','devices','cameras','zones',
                           'events','snapshots','alert_rules','compliance_checks','audit_log','api_keys','webhooks']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
  END LOOP;
END $$;

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_self ON tenants USING (id = app_tenant()) WITH CHECK (id = app_tenant());

-- The audit trail can only grow.
CREATE OR REPLACE FUNCTION audit_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only';
END $$;
CREATE TRIGGER audit_no_update BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_immutable();
