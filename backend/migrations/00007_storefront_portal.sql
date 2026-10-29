-- +goose Up
CREATE TABLE storefronts (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
  slug          citext NOT NULL UNIQUE,
  display_name  text NOT NULL,
  tagline       text,
  logo_url      text,
  cover_url     text,
  theme         jsonb NOT NULL DEFAULT '{}'::jsonb,
  whatsapp      text,
  messenger_url text,
  telegram      text,
  payment_instructions text,                              -- "bKash personal 017xxxx, send money then paste TrxID"
  payment_numbers jsonb NOT NULL DEFAULT '[]'::jsonb,     -- [{method,number,type}]
  is_published  boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER storefronts_updated BEFORE UPDATE ON storefronts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE SEQUENCE order_number_seq;
CREATE TABLE orders (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  number          integer NOT NULL,
  customer_id     uuid REFERENCES customers(id) ON DELETE SET NULL,
  conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  product_id      uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  plan_id         uuid REFERENCES product_plans(id) ON DELETE SET NULL,
  quantity        integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
  amount          bigint NOT NULL CHECK (amount >= 0),
  currency        char(3) NOT NULL DEFAULT 'BDT',
  source          text NOT NULL DEFAULT 'manual' CHECK (source IN ('storefront','inbox','manual','portal')),
  status          text NOT NULL DEFAULT 'new' CHECK (status IN ('new','awaiting_payment','paid','delivered','cancelled')),
  payment_id      uuid REFERENCES payments(id) ON DELETE SET NULL,
  subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
  customer_name   text,
  customer_phone  text,
  customer_note   text,
  created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, number)
);
CREATE INDEX orders_tenant_status_idx ON orders(tenant_id, status, created_at DESC);
CREATE TRIGGER orders_updated BEFORE UPDATE ON orders FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Passwordless customer portal.
CREATE TABLE portal_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  token_hash  text NOT NULL UNIQUE,
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX portal_tokens_customer_idx ON portal_tokens(customer_id);

CREATE TABLE imports (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('customers_csv','subscriptions_csv','chat_export')),
  file_url     text NOT NULL,
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','failed')),
  stats        jsonb NOT NULL DEFAULT '{}'::jsonb,         -- {rows, created, updated, skipped, errors:[]}
  error        text,
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz
);
CREATE INDEX imports_tenant_idx ON imports(tenant_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS imports, portal_tokens, orders, storefronts CASCADE;
DROP SEQUENCE IF EXISTS order_number_seq;
