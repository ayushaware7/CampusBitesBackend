-- Menu module. Money is stored as integer paise. Rows are soft-deleted (is_active = 0)
-- so that old orders and carts can still resolve them.

CREATE TABLE categories (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  canteen_id BIGINT UNSIGNED NOT NULL,
  name       VARCHAR(100) NOT NULL,
  sort_order INT          NOT NULL DEFAULT 0,
  is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_categories_canteen_name (canteen_id, name),
  CONSTRAINT fk_categories_canteen FOREIGN KEY (canteen_id) REFERENCES canteens (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE items (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  canteen_id   BIGINT UNSIGNED NOT NULL,
  category_id  BIGINT UNSIGNED NOT NULL,
  name         VARCHAR(100) NOT NULL,
  description  VARCHAR(500) NOT NULL DEFAULT '',
  image_url    VARCHAR(255) NOT NULL DEFAULT '',
  price_paise  BIGINT       NOT NULL DEFAULT 0,  -- base price; replaced by the variant price when variants exist
  is_veg       BOOLEAN      NOT NULL DEFAULT TRUE,
  is_special   BOOLEAN      NOT NULL DEFAULT FALSE,
  is_available BOOLEAN      NOT NULL DEFAULT TRUE,  -- "sold out" toggle
  is_active    BOOLEAN      NOT NULL DEFAULT TRUE,  -- soft delete
  sort_order   INT          NOT NULL DEFAULT 0,
  created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_items_canteen_category (canteen_id, category_id),
  CONSTRAINT fk_items_canteen  FOREIGN KEY (canteen_id)  REFERENCES canteens (id),
  CONSTRAINT fk_items_category FOREIGN KEY (category_id) REFERENCES categories (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE item_variants (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  item_id      BIGINT UNSIGNED NOT NULL,
  name         VARCHAR(50)  NOT NULL,
  price_paise  BIGINT       NOT NULL,  -- absolute price, not a delta
  is_available BOOLEAN      NOT NULL DEFAULT TRUE,
  is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
  sort_order   INT          NOT NULL DEFAULT 0,
  KEY idx_item_variants_item (item_id),
  CONSTRAINT fk_item_variants_item FOREIGN KEY (item_id) REFERENCES items (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Reusable per canteen: one "Toppings" group can be attached to many items.
CREATE TABLE addon_groups (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  canteen_id BIGINT UNSIGNED NOT NULL,
  name       VARCHAR(100) NOT NULL,
  min_select INT          NOT NULL DEFAULT 0,
  max_select INT          NOT NULL DEFAULT 1,
  sort_order INT          NOT NULL DEFAULT 0,
  is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
  KEY idx_addon_groups_canteen (canteen_id),
  CONSTRAINT fk_addon_groups_canteen FOREIGN KEY (canteen_id) REFERENCES canteens (id),
  CONSTRAINT chk_addon_groups_range CHECK (min_select >= 0 AND max_select >= 1 AND min_select <= max_select)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE addons (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  group_id     BIGINT UNSIGNED NOT NULL,
  name         VARCHAR(50)  NOT NULL,
  price_paise  BIGINT       NOT NULL DEFAULT 0,
  is_available BOOLEAN      NOT NULL DEFAULT TRUE,
  is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
  sort_order   INT          NOT NULL DEFAULT 0,
  KEY idx_addons_group (group_id),
  CONSTRAINT fk_addons_group FOREIGN KEY (group_id) REFERENCES addon_groups (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE item_addon_groups (
  item_id  BIGINT UNSIGNED NOT NULL,
  group_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (item_id, group_id),
  CONSTRAINT fk_iag_item  FOREIGN KEY (item_id)  REFERENCES items (id)        ON DELETE CASCADE,
  CONSTRAINT fk_iag_group FOREIGN KEY (group_id) REFERENCES addon_groups (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
