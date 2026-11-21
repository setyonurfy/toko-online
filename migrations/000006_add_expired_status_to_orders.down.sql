DROP INDEX idx_orders_pending_created;

UPDATE orders SET status = 'cancelled' WHERE status = 'expired';

ALTER TABLE orders DROP CONSTRAINT orders_status_check;

ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'paid', 'cancelled'));