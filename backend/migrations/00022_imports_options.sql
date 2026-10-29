-- +goose Up
-- Sprint 2 #12: imports remember what was asked (mapping/options) so the import.run job
-- can execute it and a re-run is reproducible.
ALTER TABLE imports
  ADD COLUMN upload_id  uuid,
  ADD COLUMN mapping    jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN options    jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN dry_run    boolean NOT NULL DEFAULT false,
  ADD COLUMN started_at timestamptz,
  ADD CONSTRAINT imports_upload_tenant_fk FOREIGN KEY (tenant_id, upload_id)
    REFERENCES uploads (tenant_id, id) ON DELETE SET NULL (upload_id);

-- +goose Down
ALTER TABLE imports DROP CONSTRAINT IF EXISTS imports_upload_tenant_fk;
ALTER TABLE imports DROP COLUMN IF EXISTS started_at, DROP COLUMN IF EXISTS dry_run,
  DROP COLUMN IF EXISTS options, DROP COLUMN IF EXISTS mapping, DROP COLUMN IF EXISTS upload_id;
