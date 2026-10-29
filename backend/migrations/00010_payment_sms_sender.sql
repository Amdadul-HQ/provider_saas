-- +goose Up
-- SPRINT-01-SECURITY H4: the SMS forwarder's sender id is stored and only SMS from a known
-- short code / alphanumeric sender may auto-verify a payment. Everything else is kept as
-- 'needs_review' and never auto-verifies.
ALTER TABLE payment_inbox ADD COLUMN sender text;
ALTER TABLE payment_inbox DROP CONSTRAINT IF EXISTS payment_inbox_status_check;
ALTER TABLE payment_inbox ADD CONSTRAINT payment_inbox_status_check
  CHECK (status IN ('unmatched','matched','ignored','parse_failed','needs_review'));

-- +goose Down
UPDATE payment_inbox SET status = 'unmatched' WHERE status = 'needs_review';
ALTER TABLE payment_inbox DROP CONSTRAINT IF EXISTS payment_inbox_status_check;
ALTER TABLE payment_inbox ADD CONSTRAINT payment_inbox_status_check
  CHECK (status IN ('unmatched','matched','ignored','parse_failed'));
ALTER TABLE payment_inbox DROP COLUMN IF EXISTS sender;
