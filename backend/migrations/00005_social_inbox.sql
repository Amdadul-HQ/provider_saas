-- +goose Up
-- A connected channel: Facebook page, Instagram business account, WhatsApp number, Telegram bot.
CREATE TABLE social_accounts (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  platform         text NOT NULL CHECK (platform IN ('facebook','instagram','whatsapp','telegram')),
  external_id      text NOT NULL,                       -- page id / ig id / phone_number_id / bot id
  name             text NOT NULL,
  username         text,
  avatar_url       text,
  token_secret_id  uuid REFERENCES vault_secrets(id) ON DELETE SET NULL,
  token_expires_at timestamptz,
  scopes           text[] NOT NULL DEFAULT '{}',
  webhook_verified boolean NOT NULL DEFAULT false,
  status           text NOT NULL DEFAULT 'connected' CHECK (status IN ('connected','token_expired','disconnected','error')),
  last_error       text,
  last_synced_at   timestamptz,
  connected_by     uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, platform, external_id)
);
CREATE INDEX social_accounts_tenant_idx ON social_accounts(tenant_id);
CREATE INDEX social_accounts_external_idx ON social_accounts(platform, external_id);
CREATE TRIGGER social_accounts_updated BEFORE UPDATE ON social_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE customer_identities
  ADD CONSTRAINT customer_identities_social_fk FOREIGN KEY (social_account_id) REFERENCES social_accounts(id) ON DELETE SET NULL;

CREATE TABLE conversations (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  social_account_id  uuid NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  customer_id        uuid REFERENCES customers(id) ON DELETE SET NULL,
  identity_id        uuid REFERENCES customer_identities(id) ON DELETE SET NULL,
  platform           text NOT NULL CHECK (platform IN ('facebook','instagram','whatsapp','telegram')),
  external_thread_id text NOT NULL,
  status             text NOT NULL DEFAULT 'open' CHECK (status IN ('open','pending','closed','spam')),
  assigned_to        uuid REFERENCES users(id) ON DELETE SET NULL,
  unread_count       integer NOT NULL DEFAULT 0,
  last_message_at    timestamptz,
  last_message_preview text,
  last_inbound_at    timestamptz,                       -- WhatsApp 24h window / Messenger policy
  labels             text[] NOT NULL DEFAULT '{}',
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (social_account_id, external_thread_id)
);
CREATE INDEX conversations_inbox_idx ON conversations(tenant_id, status, last_message_at DESC);
CREATE INDEX conversations_customer_idx ON conversations(customer_id);
CREATE INDEX conversations_assigned_idx ON conversations(tenant_id, assigned_to);
CREATE TRIGGER conversations_updated BEFORE UPDATE ON conversations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE messages (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  direction       text NOT NULL CHECK (direction IN ('inbound','outbound')),
  external_id     text,
  kind            text NOT NULL DEFAULT 'text' CHECK (kind IN ('text','image','video','audio','file','sticker','template','system')),
  body            text,
  attachments     jsonb NOT NULL DEFAULT '[]'::jsonb,
  sent_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,  -- outbound only
  status          text NOT NULL DEFAULT 'received' CHECK (status IN ('queued','sent','delivered','read','failed','received')),
  error           text,
  sent_at         timestamptz NOT NULL DEFAULT now(),
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX messages_external_idx ON messages(conversation_id, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX messages_conversation_idx ON messages(conversation_id, sent_at DESC);
CREATE INDEX messages_body_trgm ON messages USING gin (body gin_trgm_ops);

CREATE TABLE message_templates (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key         text NOT NULL,                             -- quick_reply.price, reminder.before_3d, delivery.default
  name        text NOT NULL,
  body        text NOT NULL,                             -- supports {{customer}} {{product}} {{expiry}} {{amount}} {{link}}
  category    text NOT NULL DEFAULT 'quick_reply' CHECK (category IN ('quick_reply','reminder','delivery','invoice','winback','other')),
  is_active   boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE TRIGGER message_templates_updated BEFORE UPDATE ON message_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Content studio
CREATE TABLE posts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  body         text NOT NULL,
  media        jsonb NOT NULL DEFAULT '[]'::jsonb,       -- [{url,type}]
  scheduled_at timestamptz,
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','publishing','published','partial','failed')),
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX posts_tenant_idx ON posts(tenant_id, status, scheduled_at);
CREATE TRIGGER posts_updated BEFORE UPDATE ON posts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE post_targets (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  post_id           uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  social_account_id uuid NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  external_post_id  text,
  status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','published','failed')),
  error             text,
  published_at      timestamptz,
  UNIQUE (post_id, social_account_id)
);

-- +goose Down
DROP TABLE IF EXISTS post_targets, posts, message_templates, messages, conversations CASCADE;
ALTER TABLE customer_identities DROP CONSTRAINT IF EXISTS customer_identities_social_fk;
DROP TABLE IF EXISTS social_accounts CASCADE;
