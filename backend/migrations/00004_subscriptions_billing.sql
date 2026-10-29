-- +goose Up
CREATE TABLE subscriptions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id    uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  product_id     uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  plan_id        uuid REFERENCES product_plans(id) ON DELETE SET NULL,
  slot_id        uuid REFERENCES slots(id) ON DELETE SET NULL,
  starts_at      timestamptz NOT NULL,
  expires_at     timestamptz NOT NULL,
  price          bigint NOT NULL CHECK (price >= 0),
  cost           bigint NOT NULL DEFAULT 0,
  currency       char(3) NOT NULL DEFAULT 'BDT',
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','expiring','expired','cancelled','replaced')),
  auto_renew     boolean NOT NULL DEFAULT false,
  delivered_at   timestamptz,
  delivery_note  text,                                 -- what was sent to customer (non-secret part)
  renewed_from_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
  sold_by        uuid REFERENCES users(id) ON DELETE SET NULL,
  notes          text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (expires_at > starts_at)
);
CREATE INDEX subscriptions_tenant_status_idx ON subscriptions(tenant_id, status);
CREATE INDEX subscriptions_customer_idx ON subscriptions(customer_id);
CREATE INDEX subscriptions_expiry_idx ON subscriptions(tenant_id, expires_at) WHERE status IN ('active','expiring');
CREATE TRIGGER subscriptions_updated BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE slots ADD CONSTRAINT slots_subscription_fk FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;

CREATE SEQUENCE invoice_number_seq;
CREATE TABLE invoices (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id   uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  number        text NOT NULL,
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','sent','partial','paid','overdue','void')),
  subtotal      bigint NOT NULL DEFAULT 0,
  discount      bigint NOT NULL DEFAULT 0,
  total         bigint NOT NULL DEFAULT 0,
  paid_amount   bigint NOT NULL DEFAULT 0,
  currency      char(3) NOT NULL DEFAULT 'BDT',
  issued_at     timestamptz NOT NULL DEFAULT now(),
  due_at        timestamptz,
  paid_at       timestamptz,
  notes         text,
  created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, number)
);
CREATE INDEX invoices_tenant_status_idx ON invoices(tenant_id, status, due_at);
CREATE INDEX invoices_customer_idx ON invoices(customer_id);
CREATE TRIGGER invoices_updated BEFORE UPDATE ON invoices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE invoice_items (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  invoice_id      uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
  description     text NOT NULL,
  quantity        integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_price      bigint NOT NULL CHECK (unit_price >= 0),
  total           bigint NOT NULL CHECK (total >= 0),
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoice_items_invoice_idx ON invoice_items(invoice_id);

CREATE TABLE payments (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id   uuid REFERENCES customers(id) ON DELETE SET NULL,
  invoice_id    uuid REFERENCES invoices(id) ON DELETE SET NULL,
  method        text NOT NULL CHECK (method IN ('bkash','nagad','rocket','upay','bank','cash','card','other')),
  trx_id        text,
  sender_number text,
  amount        bigint NOT NULL CHECK (amount > 0),
  currency      char(3) NOT NULL DEFAULT 'BDT',
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified','rejected','refunded')),
  verified_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  verified_at   timestamptz,
  verification_source text CHECK (verification_source IN ('manual','sms_match','api','storefront')),
  notes         text,
  received_at   timestamptz NOT NULL DEFAULT now(),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payments_trx_unique ON payments(tenant_id, method, trx_id) WHERE trx_id IS NOT NULL AND status <> 'rejected';
CREATE INDEX payments_tenant_status_idx ON payments(tenant_id, status, received_at DESC);
CREATE INDEX payments_invoice_idx ON payments(invoice_id);
CREATE TRIGGER payments_updated BEFORE UPDATE ON payments FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Raw incoming payment notifications (SMS forwarder / merchant API) for auto-matching.
CREATE TABLE payment_inbox (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  source        text NOT NULL CHECK (source IN ('sms','api','manual')),
  method        text CHECK (method IN ('bkash','nagad','rocket','upay','bank','other')),
  sender_number text,
  trx_id        text,
  amount        bigint,
  raw_text      text NOT NULL,
  received_at   timestamptz NOT NULL DEFAULT now(),
  matched_payment_id uuid REFERENCES payments(id) ON DELETE SET NULL,
  status        text NOT NULL DEFAULT 'unmatched' CHECK (status IN ('unmatched','matched','ignored','parse_failed')),
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_inbox_trx_idx ON payment_inbox(tenant_id, trx_id);
CREATE INDEX payment_inbox_status_idx ON payment_inbox(tenant_id, status, received_at DESC);

-- Baki khata: every debit/credit that changes customer due balance.
CREATE TABLE ledger_entries (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  entry_type  text NOT NULL CHECK (entry_type IN ('debit','credit')),   -- debit = customer owes more
  amount      bigint NOT NULL CHECK (amount > 0),
  currency    char(3) NOT NULL DEFAULT 'BDT',
  ref_type    text CHECK (ref_type IN ('invoice','payment','adjustment','refund')),
  ref_id      uuid,
  note        text,
  created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_entries_customer_idx ON ledger_entries(customer_id, created_at DESC);

-- +goose Down
ALTER TABLE slots DROP CONSTRAINT IF EXISTS slots_subscription_fk;
DROP TABLE IF EXISTS ledger_entries, payment_inbox, payments, invoice_items, invoices, subscriptions CASCADE;
DROP SEQUENCE IF EXISTS invoice_number_seq;
