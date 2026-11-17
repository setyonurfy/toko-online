CREATE TABLE orders (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id),
    status     VARCHAR(20) NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'paid', 'cancelled')),
    total      BIGINT      NOT NULL CHECK (total > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id ON orders (user_id, id DESC);

CREATE TABLE order_items (
    id          BIGSERIAL    PRIMARY KEY,
    order_id    BIGINT       NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id  BIGINT       NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    nama_produk VARCHAR(150) NOT NULL,
    harga       BIGINT       NOT NULL CHECK (harga > 0),
    jumlah      INTEGER      NOT NULL CHECK (jumlah > 0),
    subtotal    BIGINT       NOT NULL CHECK (subtotal > 0)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);