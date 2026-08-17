-- init

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE FUNCTION current_customer_id() RETURNS uuid AS $$
  SELECT NULLIF(current_setting('app.customer_id', true), '')::uuid;
$$ LANGUAGE sql STABLE;

CREATE FUNCTION is_ops() RETURNS boolean AS $$
  SELECT current_setting('app.is_ops', true) = 'true';
$$ LANGUAGE sql STABLE;

CREATE TABLE price_plans (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE price_plan_tiers (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id            uuid NOT NULL REFERENCES price_plans (id),
  sort_order         int NOT NULL,
  from_units         bigint NOT NULL CHECK (from_units >= 0),
  to_units           bigint CHECK (to_units IS NULL OR to_units > from_units),
  unit_price_micros  bigint NOT NULL CHECK (unit_price_micros >= 0),
  UNIQUE (plan_id, sort_order)
);

CREATE TABLE customers (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name           text NOT NULL,
  price_plan_id  uuid NOT NULL REFERENCES price_plans (id),
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id  uuid NOT NULL REFERENCES customers (id),
  name         text NOT NULL,
  prefix       text NOT NULL UNIQUE,
  key_hash     text NOT NULL UNIQUE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at   timestamptz
);

CREATE TABLE usage_events (
  request_id   text PRIMARY KEY,
  customer_id  uuid NOT NULL REFERENCES customers (id),
  api_key_id   uuid NOT NULL REFERENCES api_keys (id),
  endpoint     text NOT NULL,
  units        bigint NOT NULL CHECK (units > 0),
  event_ts     timestamptz NOT NULL,
  received_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX usage_events_customer_key_ts
  ON usage_events (customer_id, api_key_id, event_ts);

CREATE TABLE dirty_hours (
  customer_id  uuid NOT NULL REFERENCES customers (id),
  api_key_id   uuid NOT NULL REFERENCES api_keys (id),
  hour_bucket  timestamptz NOT NULL,
  gen          bigint NOT NULL DEFAULT 1,
  PRIMARY KEY (customer_id, api_key_id, hour_bucket)
);

CREATE TABLE usage_windows (
  customer_id  uuid NOT NULL REFERENCES customers (id),
  api_key_id   uuid NOT NULL REFERENCES api_keys (id),
  hour_bucket  timestamptz NOT NULL,
  units        bigint NOT NULL CHECK (units >= 0),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (customer_id, api_key_id, hour_bucket)
);

CREATE INDEX usage_windows_customer_hour
  ON usage_windows (customer_id, hour_bucket);

CREATE INDEX usage_windows_customer_key_hour
  ON usage_windows (customer_id, api_key_id, hour_bucket);

CREATE TABLE invoices (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id            uuid NOT NULL REFERENCES customers (id),
  period_start           timestamptz NOT NULL,
  period_end             timestamptz NOT NULL,
  status                 text NOT NULL CHECK (status IN ('issued', 'paid')),
  subtotal_micros        bigint NOT NULL CHECK (subtotal_micros >= 0),
  credit_applied_micros  bigint NOT NULL CHECK (credit_applied_micros >= 0),
  total_micros           bigint NOT NULL CHECK (total_micros >= 0),
  issued_at              timestamptz NOT NULL DEFAULT now(),
  paid_at                timestamptz,
  UNIQUE (customer_id, period_start)
);

CREATE INDEX invoices_customer_issued
  ON invoices (customer_id, issued_at DESC);

CREATE TABLE invoice_line_items (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id          uuid NOT NULL REFERENCES invoices (id),
  kind                text NOT NULL CHECK (kind IN ('tier', 'credit')),
  description         text NOT NULL,
  quantity_units      bigint NOT NULL DEFAULT 0,
  unit_price_micros   bigint NOT NULL DEFAULT 0,
  amount_micros       bigint NOT NULL,
  position            int NOT NULL
);

CREATE TABLE credit_grants (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id       uuid NOT NULL REFERENCES customers (id),
  amount_micros     bigint NOT NULL CHECK (amount_micros > 0),
  remaining_micros  bigint NOT NULL CHECK (remaining_micros >= 0),
  reason            text NOT NULL,
  actor             text NOT NULL,
  idempotency_key   text NOT NULL UNIQUE,
  created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE webhook_events (
  provider_event_id  text PRIMARY KEY,
  invoice_id         uuid NOT NULL REFERENCES invoices (id),
  received_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_entries (
  id           bigserial PRIMARY KEY,
  actor        text NOT NULL,
  action       text NOT NULL,
  entity_type  text NOT NULL,
  entity_id    uuid NOT NULL,
  before_json  jsonb,
  after_json   jsonb,
  reason       text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION audit_no_change() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_entries cannot be updated or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_entries_no_update
  BEFORE UPDATE OR DELETE ON audit_entries
  FOR EACH ROW EXECUTE FUNCTION audit_no_change();

CREATE TABLE job_locks (
  job_name         text PRIMARY KEY,
  locked_until     timestamptz,
  locked_by        text,
  last_run_at      timestamptz,
  last_success_at  timestamptz
);

INSERT INTO job_locks (job_name) VALUES ('aggregate_hours'), ('issue_invoices');

-- one plan: 10k free, next 90k at $0.001, rest at $0.0005
INSERT INTO price_plans (id, name)
VALUES ('11111111-1111-1111-1111-111111111111', 'standard');

INSERT INTO price_plan_tiers (plan_id, sort_order, from_units, to_units, unit_price_micros)
VALUES
  ('11111111-1111-1111-1111-111111111111', 1, 0,      10000,  0),
  ('11111111-1111-1111-1111-111111111111', 2, 10000,  100000, 1000),
  ('11111111-1111-1111-1111-111111111111', 3, 100000, NULL,   500);

-- rls
ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers FORCE ROW LEVEL SECURITY;
CREATE POLICY customers_tenant ON customers
  USING (is_ops() OR id = current_customer_id());

ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY api_keys_tenant ON api_keys
  USING (is_ops() OR customer_id = current_customer_id());

ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_events FORCE ROW LEVEL SECURITY;
CREATE POLICY usage_events_tenant ON usage_events
  USING (is_ops() OR customer_id = current_customer_id());

ALTER TABLE dirty_hours ENABLE ROW LEVEL SECURITY;
ALTER TABLE dirty_hours FORCE ROW LEVEL SECURITY;
CREATE POLICY dirty_hours_tenant ON dirty_hours
  USING (is_ops() OR customer_id = current_customer_id());

ALTER TABLE usage_windows ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_windows FORCE ROW LEVEL SECURITY;
CREATE POLICY usage_windows_tenant ON usage_windows
  USING (is_ops() OR customer_id = current_customer_id());

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY invoices_tenant ON invoices
  USING (is_ops() OR customer_id = current_customer_id());

ALTER TABLE invoice_line_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_line_items FORCE ROW LEVEL SECURITY;
CREATE POLICY invoice_line_items_tenant ON invoice_line_items
  USING (
    is_ops()
    OR EXISTS (
      SELECT 1 FROM invoices i
      WHERE i.id = invoice_line_items.invoice_id
        AND i.customer_id = current_customer_id()
    )
  );

ALTER TABLE credit_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE credit_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY credit_grants_tenant ON credit_grants
  USING (is_ops() OR customer_id = current_customer_id());

CREATE USER app WITH PASSWORD 'app';
GRANT CONNECT ON DATABASE billing TO app;
GRANT USAGE ON SCHEMA public TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app;
REVOKE UPDATE, DELETE ON audit_entries FROM app;
GRANT SELECT, INSERT ON audit_entries TO app;
