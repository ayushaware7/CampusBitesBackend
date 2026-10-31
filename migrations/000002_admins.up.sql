CREATE TABLE admins (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  canteen_id    BIGINT UNSIGNED NOT NULL,
  username      VARCHAR(50)  NOT NULL,
  password_hash VARCHAR(100) NOT NULL,
  created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_admins_username (username),
  CONSTRAINT fk_admins_canteen FOREIGN KEY (canteen_id) REFERENCES canteens (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
