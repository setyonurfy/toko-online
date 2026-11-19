CREATE TABLE payments (
    id         BIGSERIAL    PRIMARY KEY,
    order_id   BIGINT       NOT NULL REFERENCES orders (id),
    reference  VARCHAR(64)  NOT NULL UNIQUE,
    amount     BIGINT       NOT NULL CHECK (amount > 0),
    status     VARCHAR(20)  NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'paid', 'refund_needed')),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    paid_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_payments_order_pending ON payments (order_id) WHERE status = 'pending';

CREATE INDEX idx_payments_order_id ON payments (order_id);