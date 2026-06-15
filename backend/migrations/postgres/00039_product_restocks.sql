-- +goose Up
-- Restock tracking per (warehouse, product, supplier): a last-value table (the
-- latest restock, upserted) drives supplier-detail + product-list columns, and
-- an append-only log drives the product-detail "restock price history" tab. Both
-- are populated by PurchaseReceiptService.CreateReceipt. "created" = the PO
-- (restock order) creation time; "arrived" = the receipt's received_at. Discount
-- is stored as type+value (FIXED minor units | PERCENT basis points), mirroring
-- the purchasing/POS discount convention. Money/qty are BIGINT minor/base units.
CREATE TABLE product_last_restocks (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  warehouse_id        UUID NOT NULL REFERENCES warehouses(id),
  product_id          UUID NOT NULL REFERENCES products(id),
  supplier_id         UUID NOT NULL REFERENCES suppliers(id),
  last_price          BIGINT NOT NULL DEFAULT 0,   -- NET unit cost per base unit
  last_qty            BIGINT NOT NULL DEFAULT 0,   -- base units received
  last_discount_type  TEXT   NOT NULL DEFAULT 'FIXED',
  last_discount_value BIGINT NOT NULL DEFAULT 0,
  last_created_at     TIMESTAMPTZ NOT NULL,        -- PO (restock order) created
  last_arrived_at     TIMESTAMPTZ NOT NULL,        -- receipt received_at
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX product_last_restocks_key ON product_last_restocks(warehouse_id, product_id, supplier_id);
CREATE INDEX product_last_restocks_supplier_idx ON product_last_restocks(supplier_id, warehouse_id);
CREATE INDEX product_last_restocks_product_idx  ON product_last_restocks(product_id, warehouse_id);

CREATE TABLE product_restock_logs (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  warehouse_id       UUID NOT NULL REFERENCES warehouses(id),
  product_id         UUID NOT NULL REFERENCES products(id),
  supplier_id        UUID NOT NULL REFERENCES suppliers(id),
  price              BIGINT NOT NULL DEFAULT 0,
  qty                BIGINT NOT NULL DEFAULT 0,
  discount_type      TEXT   NOT NULL DEFAULT 'FIXED',
  discount_value     BIGINT NOT NULL DEFAULT 0,
  restock_created_at TIMESTAMPTZ NOT NULL,         -- PO created
  restock_arrived_at TIMESTAMPTZ NOT NULL,         -- receipt received_at
  receipt_id         UUID REFERENCES purchase_receipts(id),
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX product_restock_logs_product_idx  ON product_restock_logs(product_id, warehouse_id, restock_arrived_at);
CREATE INDEX product_restock_logs_supplier_idx ON product_restock_logs(supplier_id, warehouse_id);

-- +goose Down
DROP TABLE IF EXISTS product_restock_logs;
DROP TABLE IF EXISTS product_last_restocks;
