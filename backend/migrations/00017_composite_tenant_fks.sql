-- +goose Up
-- SPRINT-01-SECURITY L4: single-column FKs let a row reference another tenant's row (an id
-- existence oracle, and cross-tenant links the RLS policies cannot see). Composite FKs
-- (tenant_id, x_id) -> parent(tenant_id, id) make the database enforce same-tenant links.
-- ON DELETE SET NULL (col) (PG15+) nulls only the reference, never tenant_id.
ALTER TABLE customers     ADD CONSTRAINT customers_tenant_id_id_key     UNIQUE (tenant_id, id);
ALTER TABLE products      ADD CONSTRAINT products_tenant_id_id_key      UNIQUE (tenant_id, id);
ALTER TABLE product_plans ADD CONSTRAINT product_plans_tenant_id_id_key UNIQUE (tenant_id, id);
ALTER TABLE slots         ADD CONSTRAINT slots_tenant_id_id_key         UNIQUE (tenant_id, id);
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_tenant_id_id_key UNIQUE (tenant_id, id);

-- invoice_items -> subscriptions
ALTER TABLE invoice_items DROP CONSTRAINT invoice_items_subscription_id_fkey,
  ADD CONSTRAINT invoice_items_subscription_tenant_fk FOREIGN KEY (tenant_id, subscription_id)
    REFERENCES subscriptions (tenant_id, id) ON DELETE SET NULL (subscription_id);

-- subscriptions -> customers / products / plans / slots
ALTER TABLE subscriptions
  DROP CONSTRAINT subscriptions_customer_id_fkey,
  DROP CONSTRAINT subscriptions_product_id_fkey,
  DROP CONSTRAINT subscriptions_plan_id_fkey,
  DROP CONSTRAINT subscriptions_slot_id_fkey,
  ADD CONSTRAINT subscriptions_customer_tenant_fk FOREIGN KEY (tenant_id, customer_id)
    REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
  ADD CONSTRAINT subscriptions_product_tenant_fk FOREIGN KEY (tenant_id, product_id)
    REFERENCES products (tenant_id, id) ON DELETE RESTRICT,
  ADD CONSTRAINT subscriptions_plan_tenant_fk FOREIGN KEY (tenant_id, plan_id)
    REFERENCES product_plans (tenant_id, id) ON DELETE SET NULL (plan_id),
  ADD CONSTRAINT subscriptions_slot_tenant_fk FOREIGN KEY (tenant_id, slot_id)
    REFERENCES slots (tenant_id, id) ON DELETE SET NULL (slot_id);

-- tickets -> customers / subscriptions
ALTER TABLE tickets
  DROP CONSTRAINT tickets_customer_id_fkey,
  DROP CONSTRAINT tickets_subscription_id_fkey,
  ADD CONSTRAINT tickets_customer_tenant_fk FOREIGN KEY (tenant_id, customer_id)
    REFERENCES customers (tenant_id, id) ON DELETE CASCADE,
  ADD CONSTRAINT tickets_subscription_tenant_fk FOREIGN KEY (tenant_id, subscription_id)
    REFERENCES subscriptions (tenant_id, id) ON DELETE SET NULL (subscription_id);

-- orders -> customers / products / plans
ALTER TABLE orders
  DROP CONSTRAINT orders_customer_id_fkey,
  DROP CONSTRAINT orders_product_id_fkey,
  DROP CONSTRAINT orders_plan_id_fkey,
  ADD CONSTRAINT orders_customer_tenant_fk FOREIGN KEY (tenant_id, customer_id)
    REFERENCES customers (tenant_id, id) ON DELETE SET NULL (customer_id),
  ADD CONSTRAINT orders_product_tenant_fk FOREIGN KEY (tenant_id, product_id)
    REFERENCES products (tenant_id, id) ON DELETE RESTRICT,
  ADD CONSTRAINT orders_plan_tenant_fk FOREIGN KEY (tenant_id, plan_id)
    REFERENCES product_plans (tenant_id, id) ON DELETE SET NULL (plan_id);

-- conversations -> customers
ALTER TABLE conversations
  DROP CONSTRAINT conversations_customer_id_fkey,
  ADD CONSTRAINT conversations_customer_tenant_fk FOREIGN KEY (tenant_id, customer_id)
    REFERENCES customers (tenant_id, id) ON DELETE SET NULL (customer_id);

-- +goose Down
ALTER TABLE conversations DROP CONSTRAINT conversations_customer_tenant_fk,
  ADD CONSTRAINT conversations_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE SET NULL;
ALTER TABLE orders DROP CONSTRAINT orders_customer_tenant_fk, DROP CONSTRAINT orders_product_tenant_fk,
  DROP CONSTRAINT orders_plan_tenant_fk,
  ADD CONSTRAINT orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE SET NULL,
  ADD CONSTRAINT orders_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT,
  ADD CONSTRAINT orders_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES product_plans(id) ON DELETE SET NULL;
ALTER TABLE tickets DROP CONSTRAINT tickets_customer_tenant_fk, DROP CONSTRAINT tickets_subscription_tenant_fk,
  ADD CONSTRAINT tickets_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE,
  ADD CONSTRAINT tickets_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;
ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_customer_tenant_fk, DROP CONSTRAINT subscriptions_product_tenant_fk,
  DROP CONSTRAINT subscriptions_plan_tenant_fk, DROP CONSTRAINT subscriptions_slot_tenant_fk,
  ADD CONSTRAINT subscriptions_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT,
  ADD CONSTRAINT subscriptions_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT,
  ADD CONSTRAINT subscriptions_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES product_plans(id) ON DELETE SET NULL,
  ADD CONSTRAINT subscriptions_slot_id_fkey FOREIGN KEY (slot_id) REFERENCES slots(id) ON DELETE SET NULL;
ALTER TABLE invoice_items DROP CONSTRAINT invoice_items_subscription_tenant_fk,
  ADD CONSTRAINT invoice_items_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;
ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_tenant_id_id_key;
ALTER TABLE slots DROP CONSTRAINT slots_tenant_id_id_key;
ALTER TABLE product_plans DROP CONSTRAINT product_plans_tenant_id_id_key;
ALTER TABLE products DROP CONSTRAINT products_tenant_id_id_key;
ALTER TABLE customers DROP CONSTRAINT customers_tenant_id_id_key;
