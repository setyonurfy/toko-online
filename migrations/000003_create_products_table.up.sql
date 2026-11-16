CREATE TABLE products (
    id         BIGSERIAL    PRIMARY KEY,
    nama       VARCHAR(150) NOT NULL,
    deskripsi  TEXT         NOT NULL DEFAULT '',
    harga      BIGINT       NOT NULL CHECK (harga > 0),
    stok       INTEGER      NOT NULL CHECK (stok >= 0),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);


-- migrate create -ext sql -dir migrations -seq create_products_table