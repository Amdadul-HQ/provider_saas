-- +goose Up
CREATE TABLE products (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name          text NOT NULL,
  category      text,                                  -- streaming, ai, design, vpn, hosting...
  description   text,
  image_url     text,
  delivery_type text NOT NULL DEFAULT 'shared_slot' CHECK (delivery_type IN ('shared_slot','private_account','license_key','invite_link','manual')),
  warranty_days integer NOT NULL DEFAULT 0,
  delivery_template text,                              -- message sent on delivery; supports {{customer}} {{credentials}} {{expiry}}
  is_active     boolean NOT NULL DEFAULT true,
  sort_order    integer NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE INDEX products_tenant_idx ON products(tenant_id) WHERE deleted_at IS NULL;
CREATE TRIGGER products_updated BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Plans = sellable variants: "1 month 1 screen 350 BDT", "3 months 900 BDT".
CREATE TABLE product_plans (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  product_id    uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  name          text NOT NULL,
  duration_days integer NOT NULL CHECK (duration_days > 0),
  price         bigint NOT NULL CHECK (price >= 0),
  cost          bigint NOT NULL DEFAULT 0 CHECK (cost >= 0),
  currency      char(3) NOT NULL DEFAULT 'BDT',
  is_active     boolean NOT NULL DEFAULT true,
  sort_order    integer NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX product_plans_product_idx ON product_plans(product_id);
CREATE TRIGGER product_plans_updated BEFORE UPDATE ON product_plans FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE suppliers (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name        text NOT NULL,
  contact     text,
  notes       text,
  rating      smallint CHECK (rating BETWEEN 1 AND 5),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX suppliers_tenant_idx ON suppliers(tenant_id);
CREATE TRIGGER suppliers_updated BEFORE UPDATE ON suppliers FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Upstream account bought from a supplier; credentials live in vault_secrets.
CREATE TABLE master_accounts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  product_id     uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  supplier_id    uuid REFERENCES suppliers(id) ON DELETE SET NULL,
  label          text NOT NULL,                        -- "NF-01 (abc@gmail.com)"
  login_hint     text,                                 -- masked email, never password
  credential_secret_id uuid REFERENCES vault_secrets(id) ON DELETE SET NULL,
  total_slots    integer NOT NULL DEFAULT 1 CHECK (total_slots > 0),
  purchase_cost  bigint NOT NULL DEFAULT 0,
  currency       char(3) NOT NULL DEFAULT 'BDT',
  purchased_at   date,
  expires_at     timestamptz,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','expiring','expired','banned','replaced','retired')),
  notes          text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX master_accounts_tenant_idx ON master_accounts(tenant_id, status);
CREATE INDEX master_accounts_expiry_idx ON master_accounts(expires_at) WHERE status IN ('active','expiring');
CREATE TRIGGER master_accounts_updated BEFORE UPDATE ON master_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE slots (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  master_account_id uuid NOT NULL REFERENCES master_accounts(id) ON DELETE CASCADE,
  slot_no           integer NOT NULL,
  label             text,                              -- "Profile 3", "Seat B"
  pin               text,
  status            text NOT NULL DEFAULT 'free' CHECK (status IN ('free','reserved','assigned','banned','retired')),
  subscription_id   uuid,                              -- FK added after subscriptions exist
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (master_account_id, slot_no)
);
CREATE INDEX slots_free_idx ON slots(tenant_id, master_account_id) WHERE status = 'free';
CREATE TRIGGER slots_updated BEFORE UPDATE ON slots FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS slots, master_accounts, suppliers, product_plans, products CASCADE;
