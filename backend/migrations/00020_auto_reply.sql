-- +goose Up
-- Sprint 2 #9: inbox auto-reply rules (FAQ bot). Evaluated by the inbox.autoreply job
-- after an inbound text message is ingested; at most one reply per conversation per
-- cooldown (conversations.last_auto_reply_at).
CREATE TABLE auto_reply_rules (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name               text NOT NULL,
  keywords           text[] NOT NULL DEFAULT '{}',
  match_type         text NOT NULL DEFAULT 'contains' CHECK (match_type IN ('contains','exact','regex','any')),
  body               text,
  template_id        uuid,
  platforms          text[] NOT NULL DEFAULT '{}',           -- empty = every platform
  only_outside_hours boolean NOT NULL DEFAULT false,
  cooldown_minutes   integer NOT NULL DEFAULT 60 CHECK (cooldown_minutes BETWEEN 0 AND 10080),
  priority           integer NOT NULL DEFAULT 0,
  is_active          boolean NOT NULL DEFAULT true,
  hits               integer NOT NULL DEFAULT 0,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);
-- Composite FKs keep links inside one tenant (SECURITY L4, see 00017). A rule whose
-- template is deleted keeps its body (or is skipped when it has none).
ALTER TABLE message_templates ADD CONSTRAINT message_templates_tenant_id_id_key UNIQUE (tenant_id, id);
ALTER TABLE auto_reply_rules ADD CONSTRAINT auto_reply_rules_template_tenant_fk FOREIGN KEY (tenant_id, template_id)
  REFERENCES message_templates (tenant_id, id) ON DELETE SET NULL (template_id);
CREATE INDEX auto_reply_rules_tenant_idx ON auto_reply_rules(tenant_id, is_active, priority DESC);
CREATE TRIGGER auto_reply_rules_updated BEFORE UPDATE ON auto_reply_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE conversations ADD COLUMN last_auto_reply_at timestamptz;
-- Which rule produced an outbound auto reply (system message, sent_by_user_id IS NULL).
ALTER TABLE messages ADD COLUMN auto_reply_rule_id uuid,
  ADD CONSTRAINT messages_auto_reply_rule_tenant_fk FOREIGN KEY (tenant_id, auto_reply_rule_id)
    REFERENCES auto_reply_rules (tenant_id, id) ON DELETE SET NULL (auto_reply_rule_id);

ALTER TABLE auto_reply_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE auto_reply_rules FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auto_reply_rules
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS auto_reply_rule_id;
ALTER TABLE message_templates DROP CONSTRAINT IF EXISTS message_templates_tenant_id_id_key CASCADE;
ALTER TABLE conversations DROP COLUMN IF EXISTS last_auto_reply_at;
DROP TABLE IF EXISTS auto_reply_rules;
