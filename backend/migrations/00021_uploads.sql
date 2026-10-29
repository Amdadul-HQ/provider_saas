-- +goose Up
-- Sprint 2 #10: files uploaded through POST /uploads. storage_key is the driver key
-- (<tenant>/<random>.<ext> for public images, <tenant>/private/<random>.<ext> for import
-- documents, which GET /uploads never serves).
CREATE TABLE uploads (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  storage_key   text NOT NULL,
  url           text NOT NULL,
  kind          text NOT NULL CHECK (kind IN ('image','csv','txt','json','zip')),
  content_type  text NOT NULL,
  size_bytes    bigint NOT NULL CHECK (size_bytes >= 0),
  original_name text,
  created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, url),
  UNIQUE (tenant_id, id)
);
CREATE INDEX uploads_tenant_idx ON uploads(tenant_id, created_at DESC, id DESC);

ALTER TABLE uploads ENABLE ROW LEVEL SECURITY;
ALTER TABLE uploads FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON uploads
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE IF EXISTS uploads;
