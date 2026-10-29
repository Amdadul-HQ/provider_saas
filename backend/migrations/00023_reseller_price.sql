-- +goose Up
-- Sprint 2 #13: price a RESELLER sells a plan at (createSubscription by a reseller uses it).
ALTER TABLE product_plans ADD COLUMN reseller_price bigint CHECK (reseller_price IS NULL OR reseller_price >= 0);
-- Commission report: sales per seller in a period.
CREATE INDEX IF NOT EXISTS subscriptions_sold_by_idx ON subscriptions(tenant_id, sold_by, created_at);

-- +goose Down
DROP INDEX IF EXISTS subscriptions_sold_by_idx;
ALTER TABLE product_plans DROP COLUMN IF EXISTS reseller_price;
