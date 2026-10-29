-- +goose Up
CREATE TABLE customers (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name          text NOT NULL,
  phone         text,
  email         citext,
  whatsapp      text,
  telegram      text,
  address       text,
  tags          text[] NOT NULL DEFAULT '{}',
  notes         text,
  source        text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','facebook','instagram','whatsapp','telegram','storefront','import')),
  preferred_channel text CHECK (preferred_channel IN ('facebook','instagram','whatsapp','telegram','sms','email')),
  due_balance   bigint NOT NULL DEFAULT 0,           -- minor units; positive = customer owes
  lifetime_value bigint NOT NULL DEFAULT 0,
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('lead','active','inactive','blocked')),
  owner_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  last_contact_at timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE INDEX customers_tenant_idx ON customers(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX customers_phone_idx ON customers(tenant_id, phone);
CREATE INDEX customers_name_trgm ON customers USING gin (name gin_trgm_ops);
CREATE INDEX customers_tags_idx ON customers USING gin (tags);
CREATE TRIGGER customers_updated BEFORE UPDATE ON customers FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A customer may be reached on many channels; each channel identity maps back to one customer.
CREATE TABLE customer_identities (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id  uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  platform     text NOT NULL CHECK (platform IN ('facebook','instagram','whatsapp','telegram','sms','email')),
  external_id  text NOT NULL,            -- PSID, IGSID, wa phone, telegram chat id
  handle       text,                     -- display name / username
  avatar_url   text,
  social_account_id uuid,                -- which connected page/bot this identity came through
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, platform, external_id)
);
CREATE INDEX customer_identities_customer_idx ON customer_identities(customer_id);

CREATE TABLE customer_notes (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  author_id   uuid REFERENCES users(id) ON DELETE SET NULL,
  body        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX customer_notes_customer_idx ON customer_notes(customer_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS customer_notes, customer_identities, customers CASCADE;
