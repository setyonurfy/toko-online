ALTER TABLE orders DROP CONSTRAINT orders_status_check;

ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'paid', 'cancelled', 'expired'));

CREATE INDEX idx_orders_pending_created ON orders (created_at, id) WHERE status = 'pending';