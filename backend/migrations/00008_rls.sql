-- +goose Up
-- Defense in depth: the API sets `SET LOCAL app.tenant_id = '<uuid>'` per transaction.
-- The app connects as role `app_user` (not superuser) so policies apply.
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'tenant_members','vault_secrets','audit_logs','jobs','notifications',
    'customers','customer_identities','customer_notes',
    'products','product_plans','suppliers','master_accounts','slots',
    'subscriptions','invoices','invoice_items','payments','payment_inbox','ledger_entries',
    'social_accounts','conversations','messages','message_templates','posts','post_targets',
    'tickets','ticket_comments','reminder_rules','reminders',
    'storefronts','orders','portal_tokens','imports'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format($p$
      CREATE POLICY tenant_isolation ON %I
      USING (tenant_id IS NULL OR current_setting('app.tenant_id', true) IS NULL OR current_setting('app.tenant_id', true) = '' OR tenant_id = current_setting('app.tenant_id', true)::uuid)
      WITH CHECK (tenant_id IS NULL OR current_setting('app.tenant_id', true) IS NULL OR current_setting('app.tenant_id', true) = '' OR tenant_id = current_setting('app.tenant_id', true)::uuid)
    $p$, t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'tenant_members','vault_secrets','audit_logs','jobs','notifications',
    'customers','customer_identities','customer_notes',
    'products','product_plans','suppliers','master_accounts','slots',
    'subscriptions','invoices','invoice_items','payments','payment_inbox','ledger_entries',
    'social_accounts','conversations','messages','message_templates','posts','post_targets',
    'tickets','ticket_comments','reminder_rules','reminders',
    'storefronts','orders','portal_tokens','imports'
  ] LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;
-- +goose StatementEnd
