-- Order module. One table with a status replaces the old live/past split: "live" means
-- PLACED, PREPARING or READY. Lines are a snapshot of what was bought (names and prices at order
-- time), so later menu edits never change history. No foreign keys to other modules' tables
-- (canteen_id, item_id, ...) so this module can move to its own database later.

CREATE TABLE orders (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  canteen_id     BIGINT UNSIGNED NOT NULL,
  code           VARCHAR(8)   NOT NULL,   -- short pickup code shown to the customer
  customer_name  VARCHAR(100) NOT NULL,
  customer_phone VARCHAR(15)  NOT NULL,
  customer_email VARCHAR(100) NOT NULL DEFAULT '',
  status         VARCHAR(20)  NOT NULL,   -- PENDING_PAYMENT PLACED PREPARING READY COMPLETED REJECTED CANCELLED
  total_paise    BIGINT       NOT NULL,
  created_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_orders_canteen_code (canteen_id, code),
  KEY idx_orders_canteen_status (canteen_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_items (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_id         BIGINT UNSIGNED NOT NULL,
  item_id          BIGINT UNSIGNED NOT NULL,
  item_name        VARCHAR(100) NOT NULL,
  variant_id       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  variant_name     VARCHAR(50)  NOT NULL DEFAULT '',
  quantity         INT          NOT NULL,
  unit_price_paise BIGINT       NOT NULL,  -- variant/base price plus add-ons, for quantity 1
  KEY idx_order_items_order (order_id),
  CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_item_addons (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_item_id BIGINT UNSIGNED NOT NULL,
  addon_id      BIGINT UNSIGNED NOT NULL,
  addon_name    VARCHAR(50) NOT NULL,
  price_paise   BIGINT      NOT NULL,
  KEY idx_order_item_addons_item (order_item_id),
  CONSTRAINT fk_order_item_addons_item FOREIGN KEY (order_item_id) REFERENCES order_items (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
