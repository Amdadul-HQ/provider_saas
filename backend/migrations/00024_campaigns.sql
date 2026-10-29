-- +goose Up
-- Sprint 2 #14: win-back / upsell campaigns. A campaign targets a segment (SegmentInput
-- as JSON); sendCampaign snapshots the audience into campaign_recipients and queues one
-- campaign.send job per recipient (dedupe key campaign.send:<recipient id>).
ALTER TABLE messages ADD CONSTRAINT messages_tenant_id_id_key UNIQUE (tenant_id, id);

CREATE TABLE campaigns (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name           text NOT NULL,
  body           text NOT NULL,
  template_id    uuid,
  channel        text NOT NULL DEFAULT 'preferred' CHECK (channel IN ('preferred','facebook','instagram','whatsapp','telegram','sms','email','in_app')),
  segment        jsonb NOT NULL DEFAULT '{}'::jsonb,
  offer          jsonb,
  scheduled_at   timestamptz,
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','sending','sent','cancelled','failed')),
  audience_count integer NOT NULL DEFAULT 0,
  sent_count     integer NOT NULL DEFAULT 0,
  failed_count   integer NOT NULL DEFAULT 0,
  created_by     uuid REFERENCES users(id) ON DELETE SET NULL,
  started_at     timestamptz,
  finished_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CONSTRAINT campaigns_template_tenant_fk FOREIGN KEY (tenant_id, template_id)
    REFERENCES message_templates (tenant_id, id) ON DELETE SET NULL (template_id)
);
CREATE INDEX campaigns_tenant_idx ON campaigns(tenant_id, created_at DESC, id DESC);
CREATE TRIGGER campaigns_updated BEFORE UPDATE ON campaigns FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE campaign_recipients (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  campaign_id uuid NOT NULL,
  customer_id uuid NOT NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed','skipped','cancelled')),
  coupon_code text,
  message_id  uuid,
  error       text,
  run_at      timestamptz,
  sent_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (campaign_id, customer_id),
  CONSTRAINT campaign_recipients_campaign_tenant_fk FOREIGN KEY (tenant_id, campaign_id)
    REFERENCES campaigns (tenant_id, id) ON DELETE CASCADE,
  CONSTRAINT campaign_recipients_customer_tenant_fk FOREIGN KEY (tenant_id, customer_id)
    REFERENCES customers (tenant_id, id) ON DELETE CASCADE,
  CONSTRAINT campaign_recipients_message_tenant_fk FOREIGN KEY (tenant_id, message_id)
    REFERENCES messages (tenant_id, id) ON DELETE SET NULL (message_id)
);
CREATE INDEX campaign_recipients_status_idx ON campaign_recipients(campaign_id, status);

ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaigns FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaigns
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
ALTER TABLE campaign_recipients ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_recipients FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_recipients
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE IF EXISTS campaign_recipients, campaigns;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_tenant_id_id_key;
