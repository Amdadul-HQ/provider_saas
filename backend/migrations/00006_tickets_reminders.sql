-- +goose Up
CREATE SEQUENCE ticket_number_seq;
CREATE TABLE tickets (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  number          integer NOT NULL,
  customer_id     uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL,
  conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  kind            text NOT NULL CHECK (kind IN ('replacement','complaint','question','refund','other')),
  priority        text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
  status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','waiting_customer','resolved','closed')),
  title           text NOT NULL,
  description     text,
  reason          text,                                   -- banned, password_changed, not_working, ...
  old_slot_id     uuid REFERENCES slots(id) ON DELETE SET NULL,
  new_slot_id     uuid REFERENCES slots(id) ON DELETE SET NULL,
  assigned_to     uuid REFERENCES users(id) ON DELETE SET NULL,
  resolved_at     timestamptz,
  created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, number)
);
CREATE INDEX tickets_tenant_status_idx ON tickets(tenant_id, status, created_at DESC);
CREATE INDEX tickets_customer_idx ON tickets(customer_id);
CREATE TRIGGER tickets_updated BEFORE UPDATE ON tickets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE ticket_comments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  ticket_id  uuid NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  author_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  body       text NOT NULL,
  is_internal boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments(ticket_id, created_at);

-- Tenant-level rules: "3 days before expiry via preferred channel using template X".
CREATE TABLE reminder_rules (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name         text NOT NULL,
  trigger      text NOT NULL CHECK (trigger IN ('before_expiry','on_expiry','after_expiry','invoice_due','invoice_overdue','master_expiry')),
  offset_days  integer NOT NULL DEFAULT 0,
  send_hour    smallint NOT NULL DEFAULT 10 CHECK (send_hour BETWEEN 0 AND 23),
  channel      text NOT NULL DEFAULT 'preferred' CHECK (channel IN ('preferred','facebook','instagram','whatsapp','telegram','sms','email','in_app')),
  template_id  uuid REFERENCES message_templates(id) ON DELETE SET NULL,
  product_id   uuid REFERENCES products(id) ON DELETE CASCADE,  -- null = all products
  is_active    boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reminder_rules_tenant_idx ON reminder_rules(tenant_id) WHERE is_active;
CREATE TRIGGER reminder_rules_updated BEFORE UPDATE ON reminder_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Materialised reminder instances (one per rule × subscription/invoice).
CREATE TABLE reminders (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  rule_id         uuid REFERENCES reminder_rules(id) ON DELETE SET NULL,
  customer_id     uuid REFERENCES customers(id) ON DELETE CASCADE,
  subscription_id uuid REFERENCES subscriptions(id) ON DELETE CASCADE,
  invoice_id      uuid REFERENCES invoices(id) ON DELETE CASCADE,
  master_account_id uuid REFERENCES master_accounts(id) ON DELETE CASCADE,
  channel         text NOT NULL,
  scheduled_at    timestamptz NOT NULL,
  sent_at         timestamptz,
  status          text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','sent','failed','skipped','cancelled')),
  rendered_body   text,
  error           text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX reminders_dedupe_idx ON reminders(rule_id, subscription_id, scheduled_at) WHERE subscription_id IS NOT NULL;
CREATE INDEX reminders_due_idx ON reminders(scheduled_at) WHERE status = 'scheduled';
CREATE INDEX reminders_customer_idx ON reminders(customer_id, scheduled_at DESC);

-- +goose Down
DROP TABLE IF EXISTS reminders, reminder_rules, ticket_comments, tickets CASCADE;
DROP SEQUENCE IF EXISTS ticket_number_seq;
