-- +goose Up
-- Sprint 2 #15 (ADR-010): real-time events. Committed changes publish a tiny JSON
-- envelope on the Postgres channel `psaas_events` (NOTIFY is transactional: delivered
-- only on COMMIT, dropped on ROLLBACK, and seen by every API replica that LISTENs, no
-- matter whether the API or the worker made the change). Payloads carry ids/counters only,
-- never message bodies or secrets; the SSE handler routes them by tenant (and user).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION psaas_emit(tenant uuid, kind text, data jsonb) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('psaas_events', jsonb_build_object('t', tenant, 'k', kind, 'd', data)::text);
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION psaas_events_trigger() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME = 'messages' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'message.new', jsonb_build_object(
      'id', NEW.id, 'conversationId', NEW.conversation_id, 'direction', NEW.direction, 'status', NEW.status));
  ELSIF TG_TABLE_NAME = 'conversations' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'conversation.updated', jsonb_build_object(
      'id', NEW.id, 'status', NEW.status, 'unreadCount', NEW.unread_count, 'assignedTo', NEW.assigned_to,
      'lastMessageAt', NEW.last_message_at));
  ELSIF TG_TABLE_NAME = 'notifications' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'notification.new', jsonb_build_object(
      'id', NEW.id, 'userId', NEW.user_id, 'kind', NEW.kind, 'title', left(NEW.title, 140), 'link', NEW.link));
  ELSIF TG_TABLE_NAME = 'payments' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'payment.verified', jsonb_build_object(
      'id', NEW.id, 'invoiceId', NEW.invoice_id, 'customerId', NEW.customer_id, 'amount', NEW.amount,
      'currency', NEW.currency, 'source', NEW.verification_source));
  ELSIF TG_TABLE_NAME = 'orders' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'order.new', jsonb_build_object(
      'id', NEW.id, 'number', NEW.number, 'status', NEW.status, 'amount', NEW.amount));
  ELSIF TG_TABLE_NAME = 'campaigns' THEN
    PERFORM psaas_emit(NEW.tenant_id, 'campaign.progress', jsonb_build_object(
      'id', NEW.id, 'status', NEW.status, 'audienceCount', NEW.audience_count,
      'sentCount', NEW.sent_count, 'failedCount', NEW.failed_count));
  END IF;
  RETURN NULL;
END $$;
-- +goose StatementEnd

CREATE TRIGGER messages_events AFTER INSERT ON messages
  FOR EACH ROW EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER conversations_events AFTER UPDATE ON conversations
  FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status OR OLD.unread_count IS DISTINCT FROM NEW.unread_count
    OR OLD.assigned_to IS DISTINCT FROM NEW.assigned_to OR OLD.last_message_at IS DISTINCT FROM NEW.last_message_at
    OR OLD.labels IS DISTINCT FROM NEW.labels OR OLD.customer_id IS DISTINCT FROM NEW.customer_id)
  EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER notifications_events AFTER INSERT ON notifications
  FOR EACH ROW EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER payments_verified_ins_events AFTER INSERT ON payments
  FOR EACH ROW WHEN (NEW.status = 'verified')
  EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER payments_verified_events AFTER UPDATE OF status ON payments
  FOR EACH ROW WHEN (NEW.status = 'verified' AND OLD.status IS DISTINCT FROM NEW.status)
  EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER orders_events AFTER INSERT ON orders
  FOR EACH ROW EXECUTE FUNCTION psaas_events_trigger();
CREATE TRIGGER campaigns_events AFTER UPDATE ON campaigns
  FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status OR OLD.sent_count IS DISTINCT FROM NEW.sent_count
    OR OLD.failed_count IS DISTINCT FROM NEW.failed_count OR OLD.audience_count IS DISTINCT FROM NEW.audience_count)
  EXECUTE FUNCTION psaas_events_trigger();

-- +goose Down
DROP TRIGGER IF EXISTS campaigns_events ON campaigns;
DROP TRIGGER IF EXISTS orders_events ON orders;
DROP TRIGGER IF EXISTS payments_verified_events ON payments;
DROP TRIGGER IF EXISTS payments_verified_ins_events ON payments;
DROP TRIGGER IF EXISTS notifications_events ON notifications;
DROP TRIGGER IF EXISTS conversations_events ON conversations;
DROP TRIGGER IF EXISTS messages_events ON messages;
DROP FUNCTION IF EXISTS psaas_events_trigger();
DROP FUNCTION IF EXISTS psaas_emit(uuid, text, jsonb);
